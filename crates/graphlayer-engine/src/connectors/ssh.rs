use russh::{client, keys, ChannelMsg, Disconnect};
use serde_json::{json, Value};
use std::{collections::HashMap, path::PathBuf, sync::Arc, time::Duration};
use tokio::runtime::Runtime;

const MAX_COMMAND_BYTES: usize = 64 * 1024;
const MAX_OUTPUT_BYTES: usize = 4 * 1024 * 1024;

struct KnownHostHandler { host: String, port: u16, path: PathBuf }

impl client::Handler for KnownHostHandler {
    type Error = russh::Error;

    async fn check_server_key(&mut self, key: &keys::PublicKeyOrCertificate) -> Result<bool, Self::Error> {
        Ok(keys::check_known_hosts_path(&self.host, self.port, key.public_key(), &self.path).unwrap_or(false))
    }
}

type SshHandle = client::Handle<KnownHostHandler>;
struct ShellEntry{channel:russh::Channel<client::Msg>,pattern:String}

pub struct SessionDispatcher {
    runtime: Runtime,
    sessions: HashMap<String, SshHandle>,
    shells: HashMap<String,ShellEntry>,
}

impl SessionDispatcher {
    pub fn new() -> Self {
        Self { runtime: Runtime::new().expect("tokio runtime creation failed"), sessions: HashMap::new(), shells:HashMap::new() }
    }

    pub fn dispatch(&mut self, operation: &str, input: Value) -> Result<Value, String> {
        match operation {
            "ssh.connect" => self.connect(input),
            "ssh.execute" => self.execute(input),
            "ssh.disconnect" => self.disconnect(input),
            "ssh.close_all" => { self.sessions.clear(); Ok(json!({"closed":true})) },
            "ssh.shell_execute" => self.shell_execute(input),
            "ssh.open_shell" => self.open_shell(input),
            "ssh.device_info" => self.device_info(input),
            "ssh.get_config" => self.get_config(input),
            _ => dispatch(operation,&input),
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
        let interactive=cfg.get("interactive_shell").and_then(Value::as_bool).unwrap_or(false);
        let pattern=cfg.get("prompt_pattern").map(|v|v.as_str().unwrap_or("")).unwrap_or("").to_string();
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
            let shell=if interactive{
                let channel=handle.channel_open_session().await.map_err(|_|"ssh_shell_open_failed".to_string())?;
                channel.request_pty(true,"vt100",80,24,0,0,&[]).await.map_err(|_|"ssh_pty_failed".to_string())?;
                channel.request_shell(true).await.map_err(|_|"ssh_shell_request_failed".to_string())?;
                let drain=async{loop{if channel.wait().await.is_none(){break}}};
                let _=tokio::time::timeout(Duration::from_millis(500),drain).await;
                Some(channel)
            }else{None};
            Ok::<_, String>((handle,shell))
        });
        let (handle,shell) = result?;
        if let Some(previous) = self.sessions.insert(id.clone(), handle) {
            self.runtime.block_on(async { let mut old = previous; let _ = old.disconnect(Disconnect::ByApplication, "replaced", "English").await; });
        }
        if let Some(channel)=shell {self.shells.insert(id.clone(),ShellEntry{channel,pattern:if pattern.is_empty(){r"[>#%$]\s*$".into()}else{pattern}});}
        Ok(json!({"session_id":id,"connected":true}))
    }

    fn execute(&mut self, input: Value) -> Result<Value, String> {
        let id = string(&input, "session_id");
        let command = string(&input, "command");
        if command.is_empty() || command.len() > MAX_COMMAND_BYTES { return Err("invalid_ssh_command".into()); }
        let timeout = input.get("timeout_seconds").and_then(Value::as_u64).unwrap_or(30).clamp(1, 120);
        if self.shells.contains_key(&id){return self.shell_execute(json!({"session_id":id,"command":command,"timeout_seconds":timeout}));}
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
        self.shells.remove(&id);
        if let Some(mut handle) = self.sessions.remove(&id) {
            self.runtime.block_on(async { let _ = handle.disconnect(Disconnect::ByApplication, "", "English").await; });
        }
        Ok(json!({"session_id":id,"disconnected":true}))
    }

    fn open_shell(&mut self,input:Value)->Result<Value,String>{let mut config=input.get("config").cloned().unwrap_or_else(||json!({}));config["interactive_shell"]=json!(true);self.connect(json!({"session_id":input["session_id"],"config":config}))}
    fn shell_execute(&mut self,input:Value)->Result<Value,String>{
        let id=string(&input,"session_id");let command=string(&input,"command");if command.is_empty()||command.len()>MAX_COMMAND_BYTES{return Err("invalid_ssh_command".into())}
        let timeout=input.get("timeout_seconds").and_then(Value::as_u64).unwrap_or(30).clamp(1,120);let shell=self.shells.get_mut(&id).ok_or_else(||"ssh_shell_not_found".to_string())?;
        let prompt=shell.pattern.clone();
        self.runtime.block_on(async{
            shell.channel.data_bytes(format!("{command}\n").into_bytes()).await.map_err(|_|"ssh_shell_write_failed".to_string())?;
            let matcher=regex::Regex::new(&shell.pattern).map_err(|_|"invalid_prompt_pattern".to_string())?;let mut output=Vec::new();
            let work=async{loop{let Some(message)=shell.channel.wait().await else{break};match message{ChannelMsg::Data{data}|ChannelMsg::ExtendedData{data,..}=>{append_bounded(&mut output,&data)?;let last=String::from_utf8_lossy(&output);let line=last.lines().last().unwrap_or("").trim_end();if matcher.is_match(line){break}},_=>{}}}Ok::<(),String>(())};
            tokio::time::timeout(Duration::from_secs(timeout),work).await.map_err(|_|"ssh_command_timeout".to_string())??;
            Ok(json!({"stdout":clean_output(&String::from_utf8_lossy(&output),&prompt),"stderr":"","exit_status":0}))
        })
    }
    fn device_info(&mut self,input:Value)->Result<Value,String>{let id=string(&input,"session_id");let output=self.execute(json!({"session_id":id,"command":"show version","timeout_seconds":10}))?;let mut details=input.clone();details["output"]=output["stdout"].clone();parse_device_info(&details)}
    fn get_config(&mut self,input:Value)->Result<Value,String>{let command=config_command(&string(&input,"device_type"),&string(&input,"config_type"));if command.is_empty(){return Err("config_command_unsupported".into())}let output=self.execute(json!({"session_id":input["session_id"],"command":command,"timeout_seconds":input["timeout_seconds"].as_u64().unwrap_or(60)}))?;Ok(json!({"config":output["stdout"],"exit_status":output["exit_status"]}))}
}

pub fn dispatch(operation: &str, input: &Value) -> Result<Value, String> {
    match operation {
        "ssh.clean_output" => Ok(json!(clean_output(&string(input, "output"), &string(input, "prompt")))),
        "ssh.config_command" => Ok(json!(config_command(if string(input,"device_type").is_empty(){&string(input,"vendor")}else{&string(input,"device_type")}, &string(input, "config_type")))),
        "ssh.device_info_command" => Ok(json!(device_info_command(&string(input, "vendor")))),
        "ssh.prompt_pattern" => Ok(json!(prompt_pattern(&string(input,"device_type")))),
        "ssh.parse_output" => Ok(json!(parse_output(&string(input,"output")))),
        "ssh.device_info" => parse_device_info(input),
        "ssh.supported_commands" => Ok(supported_commands(&string(input,"device_type"))),
        "ssh.capabilities" => Ok(capabilities(&string(input,"device_type"))),
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
    let ansi=regex::Regex::new(r"\x1B(?:\[[0-?]*[ -/]*[@-~]|\][^\x07]*(?:\x07|\x1B\\))").ok();
    let mut lines=output.lines().filter_map(|line|{let line=line.trim();if line.is_empty()||line.contains("--More--")||line.contains("Press any key"){return None}Some(ansi.as_ref().map(|r|r.replace_all(line,"").to_string()).unwrap_or_else(||line.to_string()))}).collect::<Vec<_>>();
    if !prompt.is_empty(){if let Ok(re)=regex::Regex::new(prompt){while lines.last().is_some_and(|s|re.is_match(s)){lines.pop();}}}
    lines.join("\n")
}
fn config_command(device_type:&str,kind:&str)->String {
    let t=device_type.to_lowercase();
    if ["cisco_ios","cisco_asa"].contains(&t.as_str()) { if kind.eq_ignore_ascii_case("startup"){"show startup-config".into()}else{"show running-config".into()} }
    else if t=="palo_alto_pa"&&kind.eq_ignore_ascii_case("running") { "show config running".into() }
    else { String::new() }
}
fn device_info_command(vendor:&str)->String {
    let v=vendor.to_lowercase();
    if v.contains("juniper"){"show version | no-more".into()} else if v.contains("palo"){"show system info".into()} else if v.contains("forti"){"get system status".into()} else {"show version".into()}
}
fn prompt_pattern(device_type:&str)->String{match device_type.to_lowercase().as_str(){"cisco_ios"|"cisco_asa"|"cisco_nxos"=>"[>#]\\s*$".into(),"palo_alto_pa"=>">\\s*$".into(),"juniper_junos"|"juniper_srx"=>"[%>]\\s*$".into(),"fortinet_fortigate"=>"[#$]\\s*$".into(),_=>"[>#%$]\\s*$".into()}}
fn parse_output(raw:&str)->String{let ansi=regex::Regex::new(r"\x1B(?:\[[0-?]*[ -/]*[@-~]|\][^\x07]*(?:\x07|\x1B\\))").ok();raw.lines().filter_map(|line|{let line=line.trim();if line.is_empty()||line.contains("--More--")||line.contains("Press any key"){return None}Some(ansi.as_ref().map(|r|r.replace_all(line,"").to_string()).unwrap_or_else(||line.to_string()))}).collect::<Vec<_>>().join("\n")}
fn parse_device_info(input:&Value)->Result<Value,String>{let output=parse_output(&string(input,"output"));let typ=string(input,"device_type").to_lowercase();let host=string(input,"host");let vendor=match typ.as_str(){"cisco_ios"|"cisco_asa"|"cisco_nxos"=>"Cisco","palo_alto_pa"|"palo_alto_panorama"=>"Palo Alto Networks","juniper_junos"|"juniper_srx"=>"Juniper Networks","fortinet_fortigate"|"fortinet_fortimanager"=>"Fortinet","checkpoint_gaia"=>"Check Point","f5_bigip"=>"F5 Networks",_=>"Unknown"};
 let model=if typ=="cisco_ios"||typ=="cisco_asa"{regex::Regex::new(r"Cisco.*Software.*\(([^)]+)\)").ok().and_then(|r|r.captures(&output).and_then(|c|c.get(1).map(|m|m.as_str().to_string()))).unwrap_or_else(||"Unknown".into())}else if typ=="palo_alto_pa"{regex::Regex::new(r"PA-(\d+[A-Z]*-\d+[A-Z]*)").ok().and_then(|r|r.captures(&output).and_then(|c|c.get(1).map(|m|format!("PA-{}",m.as_str())))).unwrap_or_else(||"Unknown".into())}else{"Unknown".into()};
 let version=if typ=="cisco_ios"||typ=="cisco_asa"{regex::Regex::new(r"Version\s+([^\s,]+)").ok().and_then(|r|r.captures(&output).and_then(|c|c.get(1).map(|m|m.as_str().to_string()))).unwrap_or_else(||"Unknown".into())}else{"Unknown".into()};
 let os=match typ.as_str(){"cisco_ios"=>"ios","cisco_asa"=>"asa","cisco_nxos"=>"nxos","palo_alto_pa"|"palo_alto_panorama"=>"panos","juniper_junos"|"juniper_srx"=>"junos","fortinet_fortigate"|"fortinet_fortimanager"=>"fortios","checkpoint_gaia"=>"gaia","f5_bigip"=>"bigip",_=>"linux"};
 Ok(json!({"id":format!("ssh_{}_{}",host,input["port"].as_u64().unwrap_or(22)),"hostname":host,"ip_address":host,"vendor":vendor,"model":model,"os_type":os,"os_version":version,"connection_type":"ssh","capabilities":capabilities(&typ),"supported_commands":supported_commands(&typ)}))}
fn supported_commands(t:&str)->Value{let a=match t.to_lowercase().as_str(){"cisco_ios"=>vec!["show_running_config","show_startup_config","show_interfaces","show_ip_route","show_vlan","show_mac_address_table","show_version"],"cisco_asa"=>vec!["show_running_config","show_startup_config","show_interfaces","show_route","show_version"],"palo_alto_pa"=>vec!["show_running_config","show_system_info","show_interfaces","show_routing_route"],"palo_alto_panorama"=>vec!["show_panorama_status","show_devicegroups"],"generic_ssh"=>vec!["show_version","show_interfaces"],_=>Vec::<&str>::new()};json!(a)}
fn capabilities(t:&str)->Value{let a=match t.to_lowercase().as_str(){"cisco_ios"|"cisco_asa"=>vec!["running_config","startup_config","command_execution","config_commands"],"palo_alto_pa"=>vec!["running_config","command_execution"],"generic_ssh"=>vec!["command_execution"],_=>Vec::<&str>::new()};json!(a)}
