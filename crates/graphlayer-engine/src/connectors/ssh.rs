use russh::{client, keys, ChannelMsg, Disconnect, PublicKeyOrCertificate};
use serde_json::{json, Value};
use std::{collections::HashMap, path::PathBuf, sync::Arc, time::Duration};
use tokio::runtime::Runtime;

const MAX_COMMAND_BYTES: usize = 64 * 1024;
const MAX_OUTPUT_BYTES: usize = 4 * 1024 * 1024;

struct KnownHostHandler { host: String, port: u16, path: PathBuf }

impl client::Handler for KnownHostHandler {
    type Error = russh::Error;

    async fn check_server_key(&mut self, key: &PublicKeyOrCertificate) -> Result<bool, Self::Error> {
        Ok(keys::check_known_hosts_path(&self.host, self.port, key.public_key(), &self.path).unwrap_or(false))
    }
}

type SshHandle = client::Handle<KnownHostHandler>;

pub struct SessionDispatcher {
    runtime: Runtime,
    sessions: HashMap<String, SshHandle>,
}

impl SessionDispatcher {
    pub fn new() -> Self {
        Self { runtime: Runtime::new().expect("tokio runtime creation failed"), sessions: HashMap::new() }
    }

    pub fn dispatch(&mut self, operation: &str, input: Value) -> Result<Value, String> {
        match operation {
            "ssh.connect" => self.connect(input),
            "ssh.execute" => self.execute(input),
            "ssh.disconnect" => self.disconnect(input),
            "ssh.close_all" => { self.sessions.clear(); Ok(json!({"closed":true})) },
            _ => Err("unsupported_operation".into()),
        }
    }

    fn connect(&mut self, input: Value) -> Result<Value, String> {
        let id = string(&input, "session_id");
        if id.is_empty() || id.len() > 256 { return Err("invalid_session_id".into()); }
        let cfg = input.get("config").unwrap_or(&input);
        let host = string(cfg, "host");
        let username = string(cfg, "username");
        let port = cfg.get("port").and_then(Value::as_u64).unwrap_or(22).clamp(1, 65535) as u16;
        if host.is_empty() || username.is_empty() { return Err("missing_ssh_target_or_username".into()); }
        let known_hosts = known_hosts_path(cfg)?;
        let password = string(cfg, "password");
        let private_key = string(cfg, "private_key");
        let passphrase = string(cfg, "private_key_passphrase");
        if password.is_empty() && private_key.is_empty() { return Err("missing_ssh_authentication".into()); }
        let timeout = cfg.get("timeout_seconds").and_then(Value::as_u64).unwrap_or(30).clamp(1, 120);
        let handler = KnownHostHandler { host: host.clone(), port, path: known_hosts };
        let result = self.runtime.block_on(async move {
            let config = Arc::new(client::Config { inactivity_timeout: Some(Duration::from_secs(timeout)), ..Default::default() });
            let mut handle = tokio::time::timeout(Duration::from_secs(timeout), client::connect(config, (host.as_str(), port), handler))
                .await.map_err(|_| "ssh_connection_timeout".to_string())?
                .map_err(|_| "ssh_connection_failed".to_string())?;
            let auth = if !password.is_empty() {
                handle.authenticate_password(username, password).await.map_err(|_| "ssh_authentication_failed".to_string())?
            } else {
                let key = keys::decode_secret_key(&private_key, if passphrase.is_empty() { None } else { Some(&passphrase) })
                    .map_err(|_| "invalid_ssh_private_key".to_string())?;
                let hash = handle.best_supported_rsa_hash().await.map_err(|_| "ssh_authentication_failed".to_string())?.flatten();
                handle.authenticate_publickey(username, keys::PrivateKeyWithHashAlg::new(Arc::new(key), hash)).await
                    .map_err(|_| "ssh_authentication_failed".to_string())?
            };
            if !auth.success() { return Err("ssh_authentication_failed".to_string()); }
            Ok::<_, String>(handle)
        });
        let handle = result?;
        if let Some(previous) = self.sessions.insert(id.clone(), handle) {
            self.runtime.block_on(async { let mut old = previous; let _ = old.disconnect(Disconnect::ByApplication, "replaced", "English").await; });
        }
        Ok(json!({"session_id":id,"connected":true}))
    }

    fn execute(&mut self, input: Value) -> Result<Value, String> {
        let id = string(&input, "session_id");
        let command = string(&input, "command");
        if command.is_empty() || command.len() > MAX_COMMAND_BYTES { return Err("invalid_ssh_command".into()); }
        let timeout = input.get("timeout_seconds").and_then(Value::as_u64).unwrap_or(30).clamp(1, 120);
        let handle = self.sessions.get_mut(&id).ok_or_else(|| "ssh_session_not_found".to_string())?;
        self.runtime.block_on(async {
            let work = async {
                let mut channel = handle.channel_open_session().await.map_err(|_| "ssh_channel_open_failed".to_string())?;
                channel.exec(true, command.as_str()).await.map_err(|_| "ssh_exec_failed".to_string())?;
                let mut stdout = Vec::new(); let mut stderr = Vec::new(); let mut status = None;
                while let Some(message) = channel.wait().await {
                    match message {
                        ChannelMsg::Data { data } => { append_bounded(&mut stdout, &data)?; }
                        ChannelMsg::ExtendedData { data, ext: 1 } => { append_bounded(&mut stderr, &data)?; }
                        ChannelMsg::ExitStatus { exit_status } => status = Some(exit_status),
                        _ => {}
                    }
                }
                Ok::<_, String>(json!({"stdout":String::from_utf8_lossy(&stdout),"stderr":String::from_utf8_lossy(&stderr),"exit_status":status}))
            };
            tokio::time::timeout(Duration::from_secs(timeout), work).await.map_err(|_| "ssh_command_timeout".to_string())?
        })
    }

    fn disconnect(&mut self, input: Value) -> Result<Value, String> {
        let id = string(&input, "session_id");
        if let Some(mut handle) = self.sessions.remove(&id) {
            self.runtime.block_on(async { let _ = handle.disconnect(Disconnect::ByApplication, "", "English").await; });
        }
        Ok(json!({"session_id":id,"disconnected":true}))
    }
}

pub fn dispatch(operation: &str, input: &Value) -> Result<Value, String> {
    match operation {
        "ssh.clean_output" => Ok(json!(clean_output(&string(input, "output"), &string(input, "prompt")))),
        "ssh.config_command" => Ok(json!(config_command(&string(input, "vendor"), &string(input, "config_type")))),
        "ssh.device_info_command" => Ok(json!(device_info_command(&string(input, "vendor")))),
        _ => Err("stateful_ssh_operation_requires_session_mode".into()),
    }
}

fn string(v: &Value, key: &str) -> String { v.get(key).map(|x| match x { Value::String(s)=>s.clone(),Value::Number(n)=>n.to_string(),Value::Bool(b)=>b.to_string(),_=>String::new() }).unwrap_or_default() }
fn append_bounded(dst: &mut Vec<u8>, bytes: &[u8]) -> Result<(), String> { if dst.len().saturating_add(bytes.len()) > MAX_OUTPUT_BYTES { return Err("ssh_output_too_large".into()); } dst.extend_from_slice(bytes); Ok(()) }
fn known_hosts_path(cfg: &Value) -> Result<PathBuf,String> {
    let path = string(cfg,"known_hosts_path");
    let p = if !path.is_empty() { PathBuf::from(path) } else {
        let home = std::env::var_os("USERPROFILE").or_else(||std::env::var_os("HOME")).ok_or_else(||"known_hosts_unavailable".to_string())?;
        PathBuf::from(home).join(".ssh").join("known_hosts")
    };
    if !p.is_file() { return Err("known_hosts_file_missing".into()); }
    Ok(p)
}
fn clean_output(output:&str,prompt:&str)->String {
    let mut lines=output.lines().collect::<Vec<_>>();
    while lines.first().is_some_and(|s|s.trim().is_empty()){lines.remove(0);}
    while lines.last().is_some_and(|s|s.trim().is_empty()||(!prompt.is_empty()&&s.trim_end().ends_with(prompt))){lines.pop();}
    lines.join("\n")
}
fn config_command(vendor:&str,kind:&str)->String {
    let v=vendor.to_lowercase();
    if v.contains("cisco")||v.contains("arista") { if kind.eq_ignore_ascii_case("startup"){"show startup-config".into()}else{"show running-config".into()} }
    else if v.contains("juniper") { "show configuration | display set".into() }
    else if v.contains("palo") { "show config running".into() }
    else if v.contains("forti") { "show full-configuration".into() }
    else { "show running-config".into() }
}
fn device_info_command(vendor:&str)->String {
    let v=vendor.to_lowercase();
    if v.contains("juniper"){"show version | no-more".into()} else if v.contains("palo"){"show system info".into()} else if v.contains("forti"){"get system status".into()} else {"show version".into()}
}
