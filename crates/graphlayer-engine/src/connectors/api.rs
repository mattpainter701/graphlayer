use super::common::{normalize_base_url,MAX_BODY_BYTES};
use reqwest::{blocking::{Client,Response},header::{ACCEPT,CONTENT_TYPE,HeaderMap,HeaderName,HeaderValue}};
use serde_json::{json,Value};
use std::{fs,io::Read,thread,time::Duration};
use std::collections::HashMap;

pub struct SessionDispatcher{sessions:HashMap<String,Value>}
impl SessionDispatcher{
 pub fn new()->Self{Self{sessions:HashMap::new()}}
 pub fn dispatch(&mut self,op:&str,input:Value)->Result<Value,String>{
  if op=="api.connect"{let id=text(&input["session_id"]);if id.is_empty()||id.len()>256{return Err("invalid_session_id".into())}let mut cfg=input.get("config").cloned().unwrap_or_else(||json!({}));let kind=text(&input["device_type"]);authenticate(&mut cfg,&kind)?;let _=health(&json!({"config":cfg}))?;self.sessions.insert(id.clone(),json!({"config":cfg,"device_type":kind}));return Ok(json!({"session_id":id,"connected":true}))}
  if op=="api.disconnect"{let id=text(&input["session_id"]);self.sessions.remove(&id);return Ok(json!({"session_id":id,"disconnected":true}))}
  if op.starts_with("api."){let id=text(&input["session_id"]);let saved=self.sessions.get(&id).ok_or_else(||"api_session_not_found".to_string())?;let mut req=input;req["config"]=saved["config"].clone();if req.get("device_type").is_none(){req["device_type"]=saved["device_type"].clone()}return dispatch(op,&req)}
  Err("unsupported_operation".into())
 }
}

fn authenticate(cfg:&mut Value,kind:&str)->Result<(),String>{match kind.to_lowercase().as_str(){
 "cisco_fmc"=>{let result=request(&json!({"config":cfg,"method":"POST","path":"/api/fmc_platform/v1/auth/generatetoken"}))?;if result["status"].as_u64()!=Some(204){return Err("api_authentication_failed".into())}let token=result["headers"]["x-auth-access-token"].clone();if token.is_null(){return Err("api_authentication_failed".into())}cfg["auth_token"]=token;Ok(())},
 "palo_alto_panorama"=>if text(&cfg["api_key"]).is_empty(){Err("api_key_required".into())}else{Ok(())},
 "fortinet_fortimanager"=>{let body=json!({"id":1,"method":"exec","params":[{"url":"/sys/login/user","data":{"user":cfg["username"],"passwd":cfg["password"]}}]});let r=request(&json!({"config":cfg,"method":"POST","path":"/jsonrpc","body":body}))?;let token=r["body"]["session"].clone();if token.is_null(){return Err("api_authentication_failed".into())}cfg["auth_token"]=token;Ok(())},
 _=>Ok(())}}

pub fn dispatch(operation:&str,input:&Value)->Result<Value,String>{match operation{
 "api.request"=>request(input),"api.health"=>health(input),"api.device_info"=>device_info(input),"api.supported_commands"=>Ok(commands(&text(&input["device_type"]))),
 "api.vendor_operation"=>vendor_operation(input),"api.inventory"=>inventory(input),_=>Err("unsupported_operation".into())}}

fn base(config:&Value)->Result<url::Url,String>{let host=text(&config["host"]);if host.is_empty(){return Err("missing_host".into())}let port=config["port"].as_u64().filter(|p|*p>0&&*p<=65535).unwrap_or(443);let scheme=if port==80{"http"}else{"https"};let target=if host.contains("://"){host}else{format!("{scheme}://{host}:{port}")};normalize_base_url(&json!(target),"missing_host")}
fn request(input:&Value)->Result<Value,String>{
 let config=input.get("config").or_else(||input.get("connector_data")).unwrap_or(input);let mut base=base(config)?;
 let path=text(&input["path"]);if path.is_empty()||path.len()>4096||path.contains('\\'){return Err("invalid_endpoint".into())}
 let url=base.join(path.trim_start_matches('/')).map_err(|_|"invalid_endpoint".to_string())?;
 if url.scheme()!=base.scheme()||url.host_str()!=base.host_str()||url.port_or_known_default()!=base.port_or_known_default(){return Err("invalid_endpoint".into())}
 let timeout=config["timeout_seconds"].as_u64().unwrap_or(30).clamp(1,120);
 let mut builder=Client::builder().timeout(Duration::from_secs(timeout)).redirect(reqwest::redirect::Policy::none());
 if let Some(ca)=config.get("ca_cert").and_then(Value::as_str).filter(|s|!s.is_empty()){let pem=fs::read(ca).map_err(|_|"ca_certificate_unavailable".to_string())?;let cert=reqwest::Certificate::from_pem(&pem).map_err(|_|"invalid_ca_certificate".to_string())?;builder=builder.add_root_certificate(cert);}
 if let (Some(cert),Some(key))=(config.get("cert_file").and_then(Value::as_str),config.get("key_file_api").and_then(Value::as_str)){if !cert.is_empty()&&!key.is_empty(){let mut pem=fs::read(cert).map_err(|_|"client_certificate_unavailable".to_string())?;pem.extend_from_slice(&fs::read(key).map_err(|_|"client_key_unavailable".to_string())?);builder=builder.identity(reqwest::Identity::from_pem(&pem).map_err(|_|"invalid_client_identity".to_string())?);}}
 let client=builder.build().map_err(|_|"http_client_error".to_string())?;
 let method=text(&input["method"]).to_uppercase();if !matches!(method.as_str(),"GET"|"POST"|"PUT"|"PATCH"|"DELETE"|"HEAD"){return Err("unsupported_http_method".into())}
 let mut request=client.request(method.parse().map_err(|_|"unsupported_http_method".to_string())?,url).header(ACCEPT,"application/json");
 let mut headers=HeaderMap::new();if let Some(map)=config["headers"].as_object(){for(k,v)in map{let name=HeaderName::from_bytes(k.as_bytes()).map_err(|_|"invalid_header".to_string())?;let value=HeaderValue::from_str(&text(v)).map_err(|_|"invalid_header".to_string())?;headers.insert(name,value);}}
 request=request.headers(headers);
 let key=text(&config["api_key"]);let token=text(&config["token"]);let username=text(&config["username"]);let password=text(&config["password"]);
 let session_token=text(&config["auth_token"]);
 if !session_token.is_empty(){request=request.header("X-auth-access-token",session_token)}else if !key.is_empty(){request=request.header("X-API-Key",&key).header("X-PAN-KEY",key)}else if !token.is_empty(){request=request.bearer_auth(token)}else if !username.is_empty(){request=request.basic_auth(username,Some(password))}
 if let Some(query)=input.get("query").and_then(Value::as_object){let pairs=query.iter().map(|(k,v)|(k.as_str(),text(v))).collect::<Vec<_>>();request=request.query(&pairs)}
 let body=input.get("body").filter(|v|!v.is_null());if let Some(body)=body{request=request.header(CONTENT_TYPE,"application/json").json(body)}
 let retries=config["retry_count"].as_u64().unwrap_or(3).clamp(1,8);let delay=config["retry_delay_ms"].as_u64().unwrap_or(250).clamp(0,5000);
 let mut last_error="request_failed".to_string();for attempt in 0..retries{let result=request.try_clone().ok_or_else(||"request_not_replayable".to_string())?.send();match result{Ok(response)if response.status().as_u16()<500=>return response_value(response),Ok(response)=>{last_error=format!("http_status_{}",response.status().as_u16());if attempt+1<retries{thread::sleep(Duration::from_millis(delay.saturating_mul(attempt+1)));}},Err(_)=>{last_error="request_failed".into();if attempt+1<retries{thread::sleep(Duration::from_millis(delay.saturating_mul(attempt+1)));}}}}
 Err(last_error)
}
fn response_value(response:Response)->Result<Value,String>{let status=response.status().as_u16();let mut headers=serde_json::Map::new();for(k,v)in response.headers(){if let Ok(s)=v.to_str(){headers.insert(k.to_string(),json!(s));}}if response.content_length().is_some_and(|n|n>MAX_BODY_BYTES as u64){return Err("response_too_large".into())}let mut response=response;let mut bytes=Vec::new();response.take((MAX_BODY_BYTES+1)as u64).read_to_end(&mut bytes).map_err(|_|"response_read_failed".to_string())?;if bytes.len()>MAX_BODY_BYTES{return Err("response_too_large".into())}let body=serde_json::from_slice::<Value>(&bytes).unwrap_or_else(|_|json!(String::from_utf8_lossy(&bytes).to_string()));Ok(json!({"status":status,"headers":headers,"body":body}))}
fn health(i:&Value)->Result<Value,String>{let c=i.get("config").unwrap_or(i);let paths=if let Some(p)=c.get("health_endpoint").and_then(Value::as_str).filter(|s|!s.is_empty()){vec![p.to_string()]}else{vec!["/api/health".into(),"/api/version".into(),"/health".into(),"/version".into(),"/".into()]};let mut last=0;for path in paths{if let Ok(result)=request(&json!({"config":c,"method":"GET","path":path})){last=result["status"].as_u64().unwrap_or(0);if (200..300).contains(&last){return Ok(json!({"connected":true,"status":last}))}}}Err(format!("no_accessible_api_endpoints_{last}"))}
fn device_info(i:&Value)->Result<Value,String>{let c=i.get("config").unwrap_or(i);let host=text(&c["host"]);let kind=text(&i["device_type"]);let(vendor,model)=vendor_model(&kind);Ok(json!({"id":format!("api_{host}_{}",c["port"].as_u64().unwrap_or(443)),"hostname":host,"ip_address":host,"vendor":vendor,"model":model,"connection_type":"api","capabilities":capabilities(&kind)}))}
fn vendor_model(kind:&str)->(&'static str,&'static str){match kind.to_lowercase().as_str(){"cisco_fmc"=>("Cisco","Firepower Management Center"),"palo_alto_panorama"=>("Palo Alto Networks","Panorama"),"fortinet_fortimanager"=>("Fortinet","FortiManager"),"solarwinds_ncm"=>("SolarWinds","Network Configuration Manager"),_=>("Unknown","Generic API")}}
fn commands(kind:&str)->Value{let list=match kind.to_lowercase().as_str(){"cisco_fmc"=>vec!["get_device_list","get_device_config","get_security_events","get_policies"],"palo_alto_panorama"=>vec!["get_device_list","get_device_config","get_device_groups","get_templates"],"solarwinds_ncm"=>vec!["get_device_list","get_device_config","get_node_info","execute_script"],"fortinet_fortimanager"=>vec!["get_device_list"],_=>vec!["get_device_list","get_device_config"]};json!(list)}
fn capabilities(kind:&str)->Value{let mut c=vec!["api_management","device_inventory","configuration_management"];if kind.to_lowercase().contains("ncm"){c.push("network_config_management");}json!(c)}
fn vendor_operation(i:&Value)->Result<Value,String>{let c=i.get("config").unwrap_or(i);let kind=text(&i["device_type"]).to_lowercase();let op=text(&i["operation"]);let (method,path,query,body)=match (kind.as_str(),op.as_str()){
 ("cisco_fmc","get_device_list")=>("GET","/api/fmc_config/v1/domain/default/devices/devicerecords",Value::Null,Value::Null),
 ("cisco_fmc","get_security_events")=>("GET","/api/fmc_config/v1/domain/default/policy/accesspolicies",Value::Null,Value::Null),
 ("palo_alto_panorama","get_device_list")=>("GET","/api/",json!({"type":"op","cmd":"<show><devices><connected></connected></devices></show>"}),Value::Null),
 ("solarwinds_ncm","get_device_list")=>("POST","/SolarWinds/InformationService/v3/Json/Query",Value::Null,json!({"query":"SELECT NodeID, IPAddress, Caption, Vendor, MachineType, Status FROM Orion.Nodes"})),
 ("fortinet_fortimanager","get_device_list")=>("POST","/jsonrpc",Value::Null,json!({"id":1,"method":"get","params":[{"url":"/dvmdb/adom/root/device"}]})),
 _=>return Err("unsupported_vendor_operation".into())};
 request(&json!({"config":c,"method":method,"path":path,"query":query,"body":body}))
}
fn inventory(i:&Value)->Result<Value,String>{let r=vendor_operation(&json!({"config":i.get("config").unwrap_or(i),"device_type":i["device_type"],"operation":"get_device_list"}))?;let b=&r["body"];let kind=text(&i["device_type"]).to_lowercase();let rows=if kind=="cisco_fmc"{b["items"].as_array().cloned().unwrap_or_default().into_iter().map(|v|json!({"id":v["id"],"hostname":v["hostName"],"model":v["model"],"vendor":"Cisco","connection_type":"api"})).collect::<Vec<_>>()}else if kind=="solarwinds_ncm"{b["results"].as_array().cloned().unwrap_or_default().into_iter().map(|v|json!({"id":v["NodeID"].to_string(),"ip_address":v["IPAddress"],"hostname":v["Caption"],"model":v["MachineType"],"vendor":v["Vendor"],"connection_type":"api"})).collect()}else if kind=="fortinet_fortimanager"{b["result"].as_array().and_then(|a|a.first()).and_then(|v|v["data"].as_array()).cloned().unwrap_or_default().into_iter().map(|v|json!({"id":v["name"],"ip_address":v["ip"],"model":v["platform_str"],"vendor":"Fortinet","connection_type":"api"})).collect()}else{return Err("panorama_xml_inventory_requires_xml_parser".into())};Ok(json!(rows))}
fn text(v:&Value)->String{match v{Value::String(s)=>s.clone(),Value::Number(n)=>n.to_string(),Value::Bool(b)=>b.to_string(),_=>String::new()}}
