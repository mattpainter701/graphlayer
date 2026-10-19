use super::{common::MAX_BODY_BYTES, helpers};
use russh::{client, keys};
use russh_sftp::client::SftpSession;
use serde_json::{json, Value};
use std::{fs, path::{Path,PathBuf}, sync::Arc, time::Duration};
use tokio::{io::AsyncReadExt, runtime::Runtime};

const MAX_FILES:usize=5000;
const MAX_TOTAL:usize=128*1024*1024;
struct HostKey{host:String,port:u16,path:PathBuf}
impl client::Handler for HostKey{type Error=russh::Error;async fn check_server_key(&mut self,key:&keys::PublicKeyOrCertificate)->Result<bool,Self::Error>{Ok(keys::check_known_hosts_path(&self.host,self.port,&key.public_key(),&self.path).unwrap_or(false))}}

pub fn dispatch(op:&str,input:&Value)->Result<Value,String>{match op{
    "fileshare.collect"=>collect(input),"fileshare.ping"|"fileshare.connect"=>ping(input),
    "fileshare.disconnect"=>Ok(json!({"disconnected":true})),_=>helpers::dispatch(op,input)}}

fn ping(input:&Value)->Result<Value,String>{let protocol=field(input,"connector_data","payload","protocol").to_lowercase();let rt=Runtime::new().map_err(|_|"runtime_unavailable".to_string())?;if protocol=="smb"||protocol=="auto"||protocol.is_empty(){return rt.block_on(probe_smb(input))}if protocol!="ssh"{return Err("unsupported_fileshare_protocol".into())}rt.block_on(probe_sftp(input))}
fn collect(input:&Value)->Result<Value,String>{let data=input.get("connector_data").unwrap_or(input);let payload=data.get("payload").unwrap_or(&Value::Null);let protocol=strv(payload.get("protocol")).to_lowercase();let rt=Runtime::new().map_err(|_|"runtime_unavailable".to_string())?;if protocol=="smb"||protocol=="auto"||protocol.is_empty(){return rt.block_on(collect_smb(input))}if protocol!="ssh"{return Err("unsupported_fileshare_protocol".into())}
 rt.block_on(collect_sftp(input))}
async fn probe_sftp(input:&Value)->Result<Value,String>{let data=input.get("connector_data").unwrap_or(input);let creds=data.get("credentials").unwrap_or(&Value::Null);let host=strv(data.get("host"));let user=coalesce(data.get("username"),creds.get("username"));let pass=coalesce(data.get("password"),creds.get("password"));let loc=data.get("payload").and_then(|p|p.get("location")).unwrap_or(&Value::Null);let port=loc.get("port").and_then(Value::as_u64).unwrap_or(22).clamp(1,65535)as u16;let kh=known_hosts(loc)?;let handler=HostKey{host:host.clone(),port,path:kh};let config=Arc::new(client::Config::default());let mut s=client::connect(config,(host.as_str(),port),handler).await.map_err(|_|"ssh_connection_failed".to_string())?;let auth=s.authenticate_password(user,pass).await.map_err(|_|"ssh_authentication_failed".to_string())?;if !auth.success(){return Err("ssh_authentication_failed".into())}let _=s.disconnect(russh::Disconnect::ByApplication,"","English").await;Ok(json!({"connected":true}))}
async fn probe_smb(input:&Value)->Result<Value,String>{let data=input.get("connector_data").unwrap_or(input);let payload=data.get("payload").unwrap_or(&Value::Null);let loc=payload.get("location").unwrap_or(&Value::Null);let creds=data.get("credentials").unwrap_or(&Value::Null);let host=strv(data.get("host"));let share=strv(loc.get("share_name"));if host.is_empty()||share.is_empty(){return Err("missing_fileshare_target".into())}let addr=format!("{}:{}",host,loc.get("port").and_then(Value::as_u64).unwrap_or(445));let mut client=smb2::connect(&addr,&coalesce(data.get("username"),creds.get("username")),&coalesce(data.get("password"),creds.get("password"))).await.map_err(|_|"smb_connection_failed".to_string())?;let _tree=client.connect_share(&share).await.map_err(|_|"smb_share_connection_failed".to_string())?;Ok(json!({"connected":true}))}
async fn collect_smb(input:&Value)->Result<Value,String>{
 let data=input.get("connector_data").unwrap_or(input);let payload=data.get("payload").unwrap_or(&Value::Null);let loc=payload.get("location").unwrap_or(&Value::Null);
 let creds=data.get("credentials").unwrap_or(&Value::Null);let host=strv(data.get("host"));let share_name=strv(loc.get("share_name"));let user=coalesce(data.get("username"),creds.get("username"));let password=coalesce(data.get("password"),creds.get("password"));
 if host.is_empty()||share_name.is_empty(){return Err("missing_fileshare_target".into())}
 let addr=format!("{}:{}",host,loc.get("port").and_then(Value::as_u64).unwrap_or(445));let mut client=smb2::connect(&addr,&user,&password).await.map_err(|_|"smb_connection_failed".to_string())?;
 let mut share=client.connect_share(&share_name).await.map_err(|_|"smb_share_connection_failed".to_string())?;
 let root=PathBuf::from(strv(input.get("working_folder")));if root.as_os_str().is_empty(){return Err("missing_working_folder".into())}fs::create_dir_all(&root).map_err(|_|"output_directory_error".to_string())?;
 let base=normalize(&strv(loc.get("remote_path")));let recursive=payload.get("recursive").and_then(Value::as_bool).unwrap_or(true);
 let filtering=payload.get("filtering").unwrap_or(&Value::Null);let includes=filtering.get("file_patterns").and_then(Value::as_array).cloned().unwrap_or_else(||vec![json!(r".*\.cfg$"),json!(r".*\.conf$"),json!(r".*\.txt$")]);let excludes=filtering.get("exclude_patterns").and_then(Value::as_array).cloned().unwrap_or_default();
 let mut stack=vec![base.trim_start_matches('/').to_string()];let mut files=Vec::<String>::new();
 while let Some(dir)=stack.pop(){let entries=client.list_directory(&mut share,&dir).await.map_err(|_|"smb_list_failed".to_string())?;for entry in entries{let name=entry.name;if name=="."||name==".."{continue}let path=if dir.is_empty(){name.clone()}else{format!("{}/{}",dir.trim_end_matches('/'),name)};if entry.is_directory{if recursive{stack.push(path)}}else{let matched=helpers::dispatch("fileshare.matches_patterns",&json!({"filename":name,"file_patterns":includes,"exclude_patterns":excludes}))?;if matched==Value::Bool(true){files.push(path);if files.len()>MAX_FILES{return Err("too_many_files".into())}}}}}
 let mut total=0usize;let mut downloaded=0usize;
 for remote in &files{let bytes=client.read_file(&mut share,remote).await.map_err(|_|"smb_read_failed".to_string())?;if bytes.len()>MAX_BODY_BYTES{return Err("file_too_large".into())}total=total.saturating_add(bytes.len());if total>MAX_TOTAL{return Err("download_limit_exceeded".into())}
  let rel=remote.strip_prefix(base.trim_start_matches('/')).unwrap_or(remote).trim_start_matches('/');let dest=secure_child(&root,rel)?;if let Some(parent)=dest.parent(){fs::create_dir_all(parent).map_err(|_|"output_directory_error".to_string())?}fs::write(dest,bytes).map_err(|_|"file_write_failed".to_string())?;downloaded+=1;
 }
 Ok(json!({"status":"completed","files_downloaded":downloaded,"message":format!("Downloaded {downloaded} files from SMB share")}))
}
fn known_hosts(loc:&Value)->Result<PathBuf,String>{let home=std::env::var_os("USERPROFILE").or_else(||std::env::var_os("HOME")).ok_or_else(||"known_hosts_unavailable".to_string())?;let p=loc.get("known_hosts_path").map(|v| strv(Some(v))).filter(|s|!s.is_empty()).map(PathBuf::from).unwrap_or_else(||PathBuf::from(home).join(".ssh").join("known_hosts"));if !p.is_file(){return Err("known_hosts_file_missing".into())}Ok(p)}
async fn collect_sftp(input:&Value)->Result<Value,String>{
 let data=input.get("connector_data").unwrap_or(input);let payload=data.get("payload").unwrap_or(&Value::Null);let loc=payload.get("location").unwrap_or(&Value::Null);let creds=data.get("credentials").unwrap_or(&Value::Null);let host=strv(data.get("host"));let user=coalesce(data.get("username"),creds.get("username"));let pass=coalesce(data.get("password"),creds.get("password"));let port=loc.get("port").and_then(Value::as_u64).unwrap_or(22).clamp(1,65535)as u16;let timeout=payload.get("timeout_seconds").and_then(Value::as_u64).unwrap_or(30).clamp(1,120);
 if host.is_empty()||user.is_empty(){return Err("missing_fileshare_credentials".into())}
 let home=std::env::var_os("USERPROFILE").or_else(||std::env::var_os("HOME")).ok_or_else(||"known_hosts_unavailable".to_string())?;let kh=loc.get("known_hosts_path").map(|v| strv(Some(v))).filter(|s|!s.is_empty()).map(PathBuf::from).unwrap_or_else(||PathBuf::from(home).join(".ssh").join("known_hosts"));if !kh.is_file(){return Err("known_hosts_file_missing".into())}
 let handler=HostKey{host:host.clone(),port,path:kh};let cfg=Arc::new(client::Config{inactivity_timeout:Some(Duration::from_secs(timeout)),..Default::default()});
 let mut session=tokio::time::timeout(Duration::from_secs(timeout),client::connect(cfg,(host.as_str(),port),handler)).await.map_err(|_|"ssh_connection_timeout".to_string())?.map_err(|_|"ssh_connection_failed".to_string())?;
 let authenticated=if !pass.is_empty(){session.authenticate_password(user,pass).await.map_err(|_|"ssh_authentication_failed".to_string())?}else{return Err("ssh_password_required_for_fileshare".into())};
 if !authenticated.success(){return Err("ssh_authentication_failed".into())}
 let channel=session.channel_open_session().await.map_err(|_|"sftp_channel_failed".to_string())?;channel.request_subsystem(true,"sftp").await.map_err(|_|"sftp_subsystem_failed".to_string())?;let sftp=SftpSession::new(channel.into_stream()).await.map_err(|_|"sftp_initialization_failed".to_string())?;
 let root=PathBuf::from(strv(input.get("working_folder")));if root.as_os_str().is_empty(){return Err("missing_working_folder".into())}fs::create_dir_all(&root).map_err(|_|"output_directory_error".to_string())?;
 let remote=strv(loc.get("remote_path"));let base=normalize(&remote);let recursive=payload.get("recursive").and_then(Value::as_bool).unwrap_or(true);
 let includes=payload.get("filtering").and_then(|x|x.get("file_patterns")).and_then(Value::as_array).cloned().unwrap_or_else(||vec![json!(r".*\.cfg$"),json!(r".*\.conf$"),json!(r".*\.txt$")]);let excludes=payload.get("filtering").and_then(|x|x.get("exclude_patterns")).and_then(Value::as_array).cloned().unwrap_or_default();
 let mut stack=vec![if base.is_empty(){"/".to_string()}else{base.clone()}];let mut files=Vec::<String>::new();
 while let Some(dir)=stack.pop(){let entries=sftp.read_dir(&dir).await.map_err(|_|"sftp_list_failed".to_string())?;for entry in entries{let name=entry.file_name();if name=="."||name==".."{continue}let path=format!("{}/{}",dir.trim_end_matches('/'),name);let meta=sftp.metadata(&path).await.map_err(|_|"sftp_metadata_failed".to_string())?;if meta.is_dir(){if recursive{stack.push(path)}}else{let matched=helpers::dispatch("fileshare.matches_patterns",&json!({"filename":name,"file_patterns":includes,"exclude_patterns":excludes}))?;if matched==Value::Bool(true){files.push(path);if files.len()>MAX_FILES{return Err("too_many_files".into())}}}}}
 let mut total=0usize;let mut downloaded=0usize;
 for remote_file in &files{let f=sftp.open(remote_file).await.map_err(|_|"sftp_open_failed".to_string())?;let mut buf=Vec::new();f.take((MAX_BODY_BYTES+1)as u64).read_to_end(&mut buf).await.map_err(|_|"sftp_read_failed".to_string())?;if buf.len()>MAX_BODY_BYTES{return Err("file_too_large".into())}total=total.saturating_add(buf.len());if total>MAX_TOTAL{return Err("download_limit_exceeded".into())}
  let relative=remote_file.strip_prefix(&base).unwrap_or(remote_file).trim_start_matches('/');let dest=secure_child(&root,relative)?;if let Some(parent)=dest.parent(){fs::create_dir_all(parent).map_err(|_|"output_directory_error".to_string())?}fs::write(dest,buf).map_err(|_|"file_write_failed".to_string())?;downloaded+=1;
 }
 Ok(json!({"status":"completed","files_downloaded":downloaded,"message":format!("Downloaded {downloaded} files via SSH")}))
}
fn normalize(raw:&str)->String{let s=raw.trim();if s.is_empty()||s=="/"{String::new()}else if s.ends_with('/'){s[..s.len()-1].to_string()}else if s.starts_with('/'){s.to_string()}else{format!("/{s}")}}
fn secure_child(root:&Path,rel:&str)->Result<PathBuf,String>{let path=Path::new(rel);if path.components().any(|c|!matches!(c,std::path::Component::Normal(_))){return Err("invalid_remote_path".into())}Ok(root.join(path))}
fn field<'a>(v:&'a Value,a:&str,b:&str,c:&str)->String{v.get(a).and_then(|x|x.get(b)).and_then(|x|x.get(c)).map(|v| strv(Some(v))).unwrap_or_default()}
fn strv(v:Option<&Value>)->String{match v{Some(Value::String(s))=>s.clone(),Some(Value::Number(n))=>n.to_string(),Some(Value::Bool(b))=>b.to_string(),_=>String::new()}}
fn coalesce(a:Option<&Value>,b:Option<&Value>)->String{let value=strv(a);if value.is_empty(){strv(b)}else{value}}
