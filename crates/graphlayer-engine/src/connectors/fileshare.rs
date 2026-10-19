use super::{common::MAX_BODY_BYTES, helpers};
use russh::{client, keys};
use russh_sftp::client::SftpSession;
use serde_json::{json, Value};
use std::{
    fs::{self, DirBuilder, OpenOptions},
    io::Write,
    path::{Path, PathBuf},
    sync::{
        atomic::{AtomicU64, Ordering},
        Arc,
    },
    time::Duration,
};
use tokio::{io::AsyncReadExt, runtime::Runtime};

const MAX_FILES: usize = 5000;
const MAX_ENTRIES: usize = 50000;
const MAX_TOTAL: usize = 128 * 1024 * 1024;
struct HostKey {
    host: String,
    port: u16,
    path: PathBuf,
}
impl client::Handler for HostKey {
    type Error = russh::Error;
    async fn check_server_key(
        &mut self,
        key: &keys::PublicKeyOrCertificate,
    ) -> Result<bool, Self::Error> {
        Ok(
            keys::check_known_hosts_path(&self.host, self.port, &key.public_key(), &self.path)
                .unwrap_or(false),
        )
    }
}

pub fn dispatch(op: &str, input: &Value) -> Result<Value, String> {
    match op {
        "fileshare.collect" => collect(input),
        "fileshare.ping" | "fileshare.connect" => ping(input),
        "fileshare.disconnect" => Ok(json!({"disconnected":true})),
        _ => helpers::dispatch(op, input),
    }
}

fn ping(input: &Value) -> Result<Value, String> {
    let protocol = field(input, "connector_data", "payload", "protocol").to_lowercase();
    let rt = Runtime::new().map_err(|_| "runtime_unavailable".to_string())?;
    if protocol == "smb" || protocol == "auto" || protocol.is_empty() {
        return rt
            .block_on(async {
                tokio::time::timeout(operation_timeout(input), probe_smb(input)).await
            })
            .map_err(|_| "fileshare_timeout".to_string())?;
    }
    if protocol != "ssh" {
        return Err("unsupported_fileshare_protocol".into());
    }
    rt.block_on(async { tokio::time::timeout(operation_timeout(input), probe_sftp(input)).await })
        .map_err(|_| "fileshare_timeout".to_string())?
}
fn collect(input: &Value) -> Result<Value, String> {
    let data = input.get("connector_data").unwrap_or(input);
    let payload = data.get("payload").unwrap_or(&Value::Null);
    let protocol = strv(payload.get("protocol")).to_lowercase();
    let rt = Runtime::new().map_err(|_| "runtime_unavailable".to_string())?;
    if protocol == "smb" || protocol == "auto" || protocol.is_empty() {
        return rt
            .block_on(async {
                tokio::time::timeout(operation_timeout(input), collect_smb(input)).await
            })
            .map_err(|_| "fileshare_timeout".to_string())?;
    }
    if protocol != "ssh" {
        return Err("unsupported_fileshare_protocol".into());
    }
    rt.block_on(async { tokio::time::timeout(operation_timeout(input), collect_sftp(input)).await })
        .map_err(|_| "fileshare_timeout".to_string())?
}
fn operation_timeout(input: &Value) -> Duration {
    let data = input.get("connector_data").unwrap_or(input);
    let seconds = data
        .get("payload")
        .and_then(|p| p.get("timeout_seconds"))
        .and_then(Value::as_u64)
        .unwrap_or(30)
        .clamp(1, 120);
    Duration::from_secs(seconds)
}
async fn probe_sftp(input: &Value) -> Result<Value, String> {
    let data = input.get("connector_data").unwrap_or(input);
    let creds = data.get("credentials").unwrap_or(&Value::Null);
    let host = strv(data.get("host"));
    let user = coalesce(data.get("username"), creds.get("username"));
    let pass = coalesce(data.get("password"), creds.get("password"));
    let loc = data
        .get("payload")
        .and_then(|p| p.get("location"))
        .unwrap_or(&Value::Null);
    let port = loc
        .get("port")
        .and_then(Value::as_u64)
        .unwrap_or(22)
        .clamp(1, 65535) as u16;
    let kh = known_hosts(loc)?;
    let handler = HostKey {
        host: host.clone(),
        port,
        path: kh,
    };
    let config = Arc::new(client::Config::default());
    let mut s = client::connect(config, (host.as_str(), port), handler)
        .await
        .map_err(|_| "ssh_connection_failed".to_string())?;
    let auth = s
        .authenticate_password(user, pass)
        .await
        .map_err(|_| "ssh_authentication_failed".to_string())?;
    if !auth.success() {
        return Err("ssh_authentication_failed".into());
    }
    let _ = s
        .disconnect(russh::Disconnect::ByApplication, "", "English")
        .await;
    Ok(json!({"connected":true}))
}
async fn probe_smb(input: &Value) -> Result<Value, String> {
    let data = input.get("connector_data").unwrap_or(input);
    let payload = data.get("payload").unwrap_or(&Value::Null);
    let loc = payload.get("location").unwrap_or(&Value::Null);
    let creds = data.get("credentials").unwrap_or(&Value::Null);
    let host = strv(data.get("host"));
    let share = strv(loc.get("share_name"));
    if host.is_empty() || share.is_empty() {
        return Err("missing_fileshare_target".into());
    }
    let addr = format!(
        "{}:{}",
        host,
        loc.get("port").and_then(Value::as_u64).unwrap_or(445)
    );
    let mut client = smb2::connect(
        &addr,
        &coalesce(data.get("username"), creds.get("username")),
        &coalesce(data.get("password"), creds.get("password")),
    )
    .await
    .map_err(|_| "smb_connection_failed".to_string())?;
    let _tree = client
        .connect_share(&share)
        .await
        .map_err(|_| "smb_share_connection_failed".to_string())?;
    Ok(json!({"connected":true}))
}
async fn collect_smb(input: &Value) -> Result<Value, String> {
    let data = input.get("connector_data").unwrap_or(input);
    let payload = data.get("payload").unwrap_or(&Value::Null);
    let loc = payload.get("location").unwrap_or(&Value::Null);
    let creds = data.get("credentials").unwrap_or(&Value::Null);
    let host = strv(data.get("host"));
    let share_name = strv(loc.get("share_name"));
    let user = coalesce(data.get("username"), creds.get("username"));
    let password = coalesce(data.get("password"), creds.get("password"));
    if host.is_empty() || share_name.is_empty() {
        return Err("missing_fileshare_target".into());
    }
    let addr = format!(
        "{}:{}",
        host,
        loc.get("port").and_then(Value::as_u64).unwrap_or(445)
    );
    let mut client = smb2::connect(&addr, &user, &password)
        .await
        .map_err(|_| "smb_connection_failed".to_string())?;
    let mut share = client
        .connect_share(&share_name)
        .await
        .map_err(|_| "smb_share_connection_failed".to_string())?;
    let root = PathBuf::from(strv(input.get("working_folder")));
    if root.as_os_str().is_empty() {
        return Err("missing_working_folder".into());
    }
    create_private_dir_all(&root).map_err(|_| "output_directory_error".to_string())?;
    let base = normalize(&strv(loc.get("remote_path")));
    let recursive = payload
        .get("recursive")
        .and_then(Value::as_bool)
        .unwrap_or(true);
    let filtering = payload.get("filtering").unwrap_or(&Value::Null);
    let includes = filtering
        .get("file_patterns")
        .and_then(Value::as_array)
        .cloned()
        .unwrap_or_else(|| vec![json!(r".*\.cfg$"), json!(r".*\.conf$"), json!(r".*\.txt$")]);
    let excludes = filtering
        .get("exclude_patterns")
        .and_then(Value::as_array)
        .cloned()
        .unwrap_or_default();
    let mut stack = vec![base.trim_start_matches('/').to_string()];
    let mut files = Vec::<String>::new();
    let mut directories = 0usize;
    let mut entries_seen = 0usize;
    while let Some(dir) = stack.pop() {
        directories += 1;
        if directories > MAX_FILES {
            return Err("too_many_directories".into());
        }
        let Ok(entries) = client.list_directory(&mut share, &dir).await else {
            continue;
        };
        if entries.len() > MAX_ENTRIES.saturating_sub(entries_seen) {
            return Err("too_many_directory_entries".into());
        }
        entries_seen += entries.len();
        for entry in entries {
            let name = entry.name;
            if name == "." || name == ".." {
                continue;
            }
            let path = if dir.is_empty() {
                name.clone()
            } else {
                format!("{}/{}", dir.trim_end_matches('/'), name)
            };
            if entry.is_directory {
                if recursive {
                    stack.push(path)
                }
            } else {
                let matched = helpers::dispatch(
                    "fileshare.matches_patterns",
                    &json!({"filename":name,"file_patterns":includes,"exclude_patterns":excludes}),
                )?;
                if matched == Value::Bool(true) {
                    files.push(path);
                    if files.len() > MAX_FILES {
                        return Err("too_many_files".into());
                    }
                }
            }
        }
    }
    let mut total = 0usize;
    let mut downloaded = 0usize;
    let mut failed = 0usize;
    for remote in &files {
        let Ok(mut download) = client.download(&share, remote).await else {
            failed += 1;
            continue;
        };
        if download.size() > MAX_BODY_BYTES as u64 {
            return Err("file_too_large".into());
        }
        let mut bytes = Vec::with_capacity(download.size() as usize);
        while let Some(chunk) = download.next_chunk().await {
            let chunk = chunk.map_err(|_| "smb_download_failed".to_string())?;
            if bytes.len().saturating_add(chunk.len()) > MAX_BODY_BYTES {
                return Err("file_too_large".into());
            }
            bytes.extend_from_slice(&chunk)
        }
        total = total.saturating_add(bytes.len());
        if total > MAX_TOTAL {
            return Err("download_limit_exceeded".into());
        }
        let rel = remote
            .strip_prefix(base.trim_start_matches('/'))
            .unwrap_or(remote)
            .trim_start_matches('/');
        if secure_write(&root, rel, &bytes).is_err() {
            failed += 1;
            continue;
        }
        downloaded += 1;
    }
    Ok(
        json!({"status":"completed","files_downloaded":downloaded,"files_failed":failed,"message":format!("Downloaded {downloaded} files from SMB share")}),
    )
}
fn known_hosts(loc: &Value) -> Result<PathBuf, String> {
    let home = std::env::var_os("USERPROFILE")
        .or_else(|| std::env::var_os("HOME"))
        .ok_or_else(|| "known_hosts_unavailable".to_string())?;
    let p = loc
        .get("known_hosts_path")
        .map(|v| strv(Some(v)))
        .filter(|s| !s.is_empty())
        .map(PathBuf::from)
        .unwrap_or_else(|| PathBuf::from(home).join(".ssh").join("known_hosts"));
    if !p.is_file() {
        return Err("known_hosts_file_missing".into());
    }
    Ok(p)
}
async fn collect_sftp(input: &Value) -> Result<Value, String> {
    let data = input.get("connector_data").unwrap_or(input);
    let payload = data.get("payload").unwrap_or(&Value::Null);
    let loc = payload.get("location").unwrap_or(&Value::Null);
    let creds = data.get("credentials").unwrap_or(&Value::Null);
    let host = strv(data.get("host"));
    let user = coalesce(data.get("username"), creds.get("username"));
    let pass = coalesce(data.get("password"), creds.get("password"));
    let port = loc
        .get("port")
        .and_then(Value::as_u64)
        .unwrap_or(22)
        .clamp(1, 65535) as u16;
    let timeout = payload
        .get("timeout_seconds")
        .and_then(Value::as_u64)
        .unwrap_or(30)
        .clamp(1, 120);
    if host.is_empty() || user.is_empty() {
        return Err("missing_fileshare_credentials".into());
    }
    let home = std::env::var_os("USERPROFILE")
        .or_else(|| std::env::var_os("HOME"))
        .ok_or_else(|| "known_hosts_unavailable".to_string())?;
    let kh = loc
        .get("known_hosts_path")
        .map(|v| strv(Some(v)))
        .filter(|s| !s.is_empty())
        .map(PathBuf::from)
        .unwrap_or_else(|| PathBuf::from(home).join(".ssh").join("known_hosts"));
    if !kh.is_file() {
        return Err("known_hosts_file_missing".into());
    }
    let handler = HostKey {
        host: host.clone(),
        port,
        path: kh,
    };
    let cfg = Arc::new(client::Config {
        inactivity_timeout: Some(Duration::from_secs(timeout)),
        ..Default::default()
    });
    let mut session = tokio::time::timeout(
        Duration::from_secs(timeout),
        client::connect(cfg, (host.as_str(), port), handler),
    )
    .await
    .map_err(|_| "ssh_connection_timeout".to_string())?
    .map_err(|_| "ssh_connection_failed".to_string())?;
    let authenticated = if !pass.is_empty() {
        session
            .authenticate_password(user, pass)
            .await
            .map_err(|_| "ssh_authentication_failed".to_string())?
    } else {
        return Err("ssh_password_required_for_fileshare".into());
    };
    if !authenticated.success() {
        return Err("ssh_authentication_failed".into());
    }
    let channel = session
        .channel_open_session()
        .await
        .map_err(|_| "sftp_channel_failed".to_string())?;
    channel
        .request_subsystem(true, "sftp")
        .await
        .map_err(|_| "sftp_subsystem_failed".to_string())?;
    let sftp = SftpSession::new(channel.into_stream())
        .await
        .map_err(|_| "sftp_initialization_failed".to_string())?;
    let root = PathBuf::from(strv(input.get("working_folder")));
    if root.as_os_str().is_empty() {
        return Err("missing_working_folder".into());
    }
    create_private_dir_all(&root).map_err(|_| "output_directory_error".to_string())?;
    let remote = strv(loc.get("remote_path"));
    let base = normalize(&remote);
    let recursive = payload
        .get("recursive")
        .and_then(Value::as_bool)
        .unwrap_or(true);
    let includes = payload
        .get("filtering")
        .and_then(|x| x.get("file_patterns"))
        .and_then(Value::as_array)
        .cloned()
        .unwrap_or_else(|| vec![json!(r".*\.cfg$"), json!(r".*\.conf$"), json!(r".*\.txt$")]);
    let excludes = payload
        .get("filtering")
        .and_then(|x| x.get("exclude_patterns"))
        .and_then(Value::as_array)
        .cloned()
        .unwrap_or_default();
    let mut stack = vec![if base.is_empty() {
        "/".to_string()
    } else {
        base.clone()
    }];
    let mut files = Vec::<String>::new();
    let mut directories = 0usize;
    let mut entries_seen = 0usize;
    while let Some(dir) = stack.pop() {
        directories += 1;
        if directories > MAX_FILES {
            return Err("too_many_directories".into());
        }
        let Ok(entries) = sftp.read_dir(&dir).await else {
            continue;
        };
        for entry in entries {
            entries_seen = entries_seen.saturating_add(1);
            if entries_seen > MAX_ENTRIES {
                return Err("too_many_directory_entries".into());
            }
            let name = entry.file_name();
            if name == "." || name == ".." {
                continue;
            }
            let path = format!("{}/{}", dir.trim_end_matches('/'), name);
            let Ok(meta) = sftp.metadata(&path).await else {
                continue;
            };
            if meta.is_dir() {
                if recursive {
                    stack.push(path)
                }
            } else {
                let matched = helpers::dispatch(
                    "fileshare.matches_patterns",
                    &json!({"filename":name,"file_patterns":includes,"exclude_patterns":excludes}),
                )?;
                if matched == Value::Bool(true) {
                    files.push(path);
                    if files.len() > MAX_FILES {
                        return Err("too_many_files".into());
                    }
                }
            }
        }
    }
    let mut total = 0usize;
    let mut downloaded = 0usize;
    let mut failed = 0usize;
    for remote_file in &files {
        let Ok(f) = sftp.open(remote_file).await else {
            failed += 1;
            continue;
        };
        let mut buf = Vec::new();
        if f.take((MAX_BODY_BYTES + 1) as u64)
            .read_to_end(&mut buf)
            .await
            .is_err()
        {
            failed += 1;
            continue;
        }
        if buf.len() > MAX_BODY_BYTES {
            return Err("file_too_large".into());
        }
        total = total.saturating_add(buf.len());
        if total > MAX_TOTAL {
            return Err("download_limit_exceeded".into());
        }
        let relative = remote_file
            .strip_prefix(&base)
            .unwrap_or(remote_file)
            .trim_start_matches('/');
        if secure_write(&root, relative, &buf).is_err() {
            failed += 1;
            continue;
        }
        downloaded += 1;
    }
    Ok(
        json!({"status":"completed","files_downloaded":downloaded,"files_failed":failed,"message":format!("Downloaded {downloaded} files via SSH")}),
    )
}
fn normalize(raw: &str) -> String {
    let s = raw.trim();
    if s.is_empty() || s == "/" {
        String::new()
    } else if let Some(stripped) = s.strip_suffix('/') {
        stripped.to_string()
    } else if s.starts_with('/') {
        s.to_string()
    } else {
        format!("/{s}")
    }
}
pub(crate) fn secure_write(root: &Path, rel: &str, bytes: &[u8]) -> Result<(), String> {
    let dest = secure_child(root, rel)?;
    match fs::symlink_metadata(&dest) {
        Ok(meta) if meta.file_type().is_symlink() || !meta.is_file() => {
            return Err("unsafe_output_path".into())
        }
        Ok(_) => {}
        Err(e) if e.kind() == std::io::ErrorKind::NotFound => {}
        Err(_) => return Err("output_file_error".to_string()),
    }
    static NEXT_TEMP: AtomicU64 = AtomicU64::new(0);
    let parent = dest
        .parent()
        .ok_or_else(|| "output_file_error".to_string())?;
    let mut temp = None;
    for _ in 0..16 {
        let id = NEXT_TEMP.fetch_add(1, Ordering::Relaxed);
        let candidate = parent.join(format!(".graphlayer-{}-{id}.tmp", std::process::id()));
        let mut options = OpenOptions::new();
        options.write(true).create_new(true);
        #[cfg(unix)]
        {
            use std::os::unix::fs::OpenOptionsExt;
            options.mode(0o600);
        }
        match options.open(&candidate) {
            Ok(mut file) => {
                let write_result = file.write_all(bytes).and_then(|()| file.sync_all());
                drop(file);
                if write_result.is_err() {
                    let _ = fs::remove_file(&candidate);
                    return Err("output_file_error".into());
                }
                temp = Some(candidate);
                break;
            }
            Err(error) if error.kind() == std::io::ErrorKind::AlreadyExists => continue,
            Err(_) => return Err("output_file_error".into()),
        }
    }
    let temp = temp.ok_or_else(|| "output_file_error".to_string())?;
    if fs::rename(&temp, &dest).is_err() {
        let _ = fs::remove_file(&temp);
        return Err("output_file_error".into());
    }
    Ok(())
}
fn secure_child(root: &Path, rel: &str) -> Result<PathBuf, String> {
    let path = Path::new(rel);
    let components = path.components().collect::<Vec<_>>();
    if components.is_empty()
        || components
            .iter()
            .any(|c| !matches!(c, std::path::Component::Normal(_)))
    {
        return Err("invalid_remote_path".into());
    }
    let canonical_root =
        fs::canonicalize(root).map_err(|_| "output_directory_error".to_string())?;
    let mut parent = canonical_root.clone();
    for component in components.iter().take(components.len() - 1) {
        let std::path::Component::Normal(name) = component else {
            return Err("invalid_remote_path".into());
        };
        parent.push(name);
        match fs::symlink_metadata(&parent) {
            Ok(meta) if meta.file_type().is_symlink() => return Err("unsafe_output_path".into()),
            Ok(meta) if meta.is_dir() => {}
            Ok(_) => return Err("unsafe_output_path".into()),
            Err(e) if e.kind() == std::io::ErrorKind::NotFound => {
                create_private_dir(&parent).map_err(|_| "output_directory_error".to_string())?
            }
            Err(_) => return Err("output_directory_error".into()),
        }
    }
    let resolved_parent =
        fs::canonicalize(&parent).map_err(|_| "output_directory_error".to_string())?;
    if !resolved_parent.starts_with(&canonical_root) {
        return Err("unsafe_output_path".into());
    }
    let filename = match components.last() {
        Some(std::path::Component::Normal(name)) => *name,
        _ => return Err("invalid_remote_path".into()),
    };
    Ok(resolved_parent.join(filename))
}
fn create_private_dir(path: &Path) -> std::io::Result<()> {
    let mut builder = DirBuilder::new();
    #[cfg(unix)]
    {
        use std::os::unix::fs::DirBuilderExt;
        builder.mode(0o700);
    }
    builder.create(path)
}
pub(crate) fn create_private_dir_all(path: &Path) -> std::io::Result<()> {
    let mut builder = DirBuilder::new();
    #[cfg(unix)]
    {
        use std::os::unix::fs::DirBuilderExt;
        builder.mode(0o700);
    }
    builder.recursive(true).create(path)
}
pub(crate) fn prepare_private_output_directory(path: &Path) -> Result<PathBuf, String> {
    if path.as_os_str().is_empty() {
        return Err("invalid_output_directory".into());
    }
    let mut current = if path.is_absolute() {
        PathBuf::new()
    } else {
        PathBuf::from(".")
    };
    for component in path.components() {
        match component {
            std::path::Component::Prefix(prefix) => current.push(prefix.as_os_str()),
            std::path::Component::RootDir => current.push(component.as_os_str()),
            std::path::Component::CurDir => continue,
            std::path::Component::ParentDir => current.push(".."),
            std::path::Component::Normal(name) => current.push(name),
        }
        if matches!(component, std::path::Component::Prefix(_)) {
            continue;
        }
        match fs::symlink_metadata(&current) {
            Ok(meta) if meta.file_type().is_symlink() || !meta.is_dir() => {
                return Err("invalid_output_directory".into())
            }
            Ok(_) => {}
            Err(error) if error.kind() == std::io::ErrorKind::NotFound => {
                create_private_dir(&current).map_err(|_| "output_directory_error".to_string())?;
            }
            Err(_) => return Err("output_directory_error".into()),
        }
    }
    fs::canonicalize(&current).map_err(|_| "output_directory_error".to_string())
}
fn field(v: &Value, a: &str, b: &str, c: &str) -> String {
    v.get(a)
        .and_then(|x| x.get(b))
        .and_then(|x| x.get(c))
        .map(|v| strv(Some(v)))
        .unwrap_or_default()
}
fn strv(v: Option<&Value>) -> String {
    match v {
        Some(Value::String(s)) => s.clone(),
        Some(Value::Number(n)) => n.to_string(),
        Some(Value::Bool(b)) => b.to_string(),
        _ => String::new(),
    }
}
fn coalesce(a: Option<&Value>, b: Option<&Value>) -> String {
    let value = strv(a);
    if value.is_empty() {
        strv(b)
    } else {
        value
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    fn temp_root() -> PathBuf {
        let path = std::env::temp_dir().join(format!(
            "graphlayer-fileshare-{}-{}",
            std::process::id(),
            std::time::SystemTime::now()
                .duration_since(std::time::UNIX_EPOCH)
                .unwrap()
                .as_nanos()
        ));
        fs::create_dir_all(&path).unwrap();
        path
    }
    #[test]
    fn safe_output_creates_nested_files_and_rejects_traversal() {
        let root = temp_root();
        let target = secure_child(&root, "nested/config.txt").unwrap();
        assert!(target.starts_with(fs::canonicalize(&root).unwrap()));
        assert!(target.parent().unwrap().is_dir());
        assert_eq!(secure_write(&root, "nested/config.txt", b"config"), Ok(()));
        assert_eq!(fs::read(target).unwrap(), b"config");
        assert!(secure_child(&root, "../escape.txt").is_err());
        let _ = fs::remove_dir_all(root);
    }
    #[cfg(unix)]
    #[test]
    fn newly_created_acquisition_files_and_directories_are_private() {
        use std::os::unix::fs::PermissionsExt;
        let root = std::env::temp_dir().join(format!(
            "graphlayer-private-{}-{}",
            std::process::id(),
            std::time::SystemTime::now()
                .duration_since(std::time::UNIX_EPOCH)
                .unwrap()
                .as_nanos()
        ));
        create_private_dir_all(&root).unwrap();
        secure_write(&root, "nested/config.txt", b"secret").unwrap();
        let root_mode = fs::metadata(&root).unwrap().permissions().mode() & 0o777;
        let dir_mode = fs::metadata(root.join("nested"))
            .unwrap()
            .permissions()
            .mode()
            & 0o777;
        let file_mode = fs::metadata(root.join("nested/config.txt"))
            .unwrap()
            .permissions()
            .mode()
            & 0o777;
        assert_eq!(root_mode, 0o700);
        assert_eq!(dir_mode, 0o700);
        assert_eq!(file_mode, 0o600);
        let _ = fs::remove_dir_all(root);
    }
    #[cfg(unix)]
    #[test]
    fn safe_output_rejects_symlinked_parent() {
        use std::os::unix::fs::symlink;
        let root = temp_root();
        let outside = temp_root();
        symlink(&outside, root.join("link")).unwrap();
        assert!(secure_child(&root, "link/escape.txt").is_err());
        let _ = fs::remove_dir_all(root);
        let _ = fs::remove_dir_all(outside);
    }
}
