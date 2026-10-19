use super::common::*;
use reqwest::blocking::Client;
use reqwest::header::{ACCEPT, AUTHORIZATION};
use serde_json::{json, Value};
use std::time::Duration;

fn config(input: &Value) -> &Value { input.get("connector_data").unwrap_or(&Value::Null) }
fn payload(input: &Value) -> &Value { config(input).get("payload").unwrap_or(&Value::Null) }
fn setting(input: &Value, key: &str, default: &str) -> String {
    let v = payload(input).get(key).filter(|v| !v.is_null()).unwrap_or(&Value::Null);
    if text(v).is_empty() { default.to_string() } else { text(v) }
}
fn endpoint(input: &Value, key: &str, fallback: &str) -> String {
    setting(input, key, fallback).trim_start_matches('/').to_string()
}
fn base_url(input: &Value) -> Result<url::Url, String> {
    let d = config(input);
    let p = payload(input);
    normalize_base_url(p.get("instance_url").or_else(|| d.get("instance_url")).or_else(|| d.get("host")).unwrap_or(&Value::Null), "instance_url or host is required for NetBox")
}
fn token(input: &Value) -> String {
    let d = config(input); let p = payload(input); let c = d.get("credentials").unwrap_or(&Value::Null);
    for v in [p.get("api_token"), c.get("api_token"), c.get("token"), d.get("api_token")].into_iter().flatten() {
        if !text(v).is_empty() { return text(v); }
    }
    String::new()
}
fn client(input: &Value) -> Result<Client, String> {
    let timeout = setting(input, "timeout_seconds", "30").parse::<u64>().unwrap_or(30).clamp(1, 120);
    let verify = payload(input).get("verify_ssl").and_then(Value::as_bool).unwrap_or(true);
    Client::builder().timeout(Duration::from_secs(timeout)).danger_accept_invalid_certs(!verify)
        .redirect(reqwest::redirect::Policy::none()).build().map_err(|_| "http_client_unavailable".to_string())
}
fn request_json(input: &Value, path: &str, params: &[(&str, String)]) -> Result<Value, String> {
    let mut base = base_url(input)?;
    if !base.path().ends_with('/') { let path = format!("{}/", base.path()); base.set_path(&path); }
    let mut url = base.join(path).map_err(|_| "invalid_target_url".to_string())?;
    // Connector endpoints must stay on the configured authority.
    if url.scheme() != base.scheme() || url.host_str() != base.host_str() || url.port_or_known_default() != base.port_or_known_default() {
        return Err("invalid_endpoint".to_string());
    }
    for (key, value) in params { url.query_pairs_mut().append_pair(key, value); }
    let client = client(input)?;
    let mut req = client.get(url).header(ACCEPT, "application/json");
    let secret = token(input);
    if !secret.is_empty() { req = req.header(AUTHORIZATION, format!("Token {secret}")); }
    let mut response = req.send().map_err(|_| "NetBox API request failed".to_string())?;
    if !response.status().is_success() { return Err(format!("NetBox API returned HTTP {}", response.status().as_u16())); }
    if response.content_length().is_some_and(|length| length > MAX_BODY_BYTES as u64) { return Err("response_too_large".into()); }
    let mut bytes = Vec::new();
    response.take((MAX_BODY_BYTES + 1) as u64).read_to_end(&mut bytes).map_err(|_| "NetBox API response read failed".to_string())?;
    if bytes.len() > MAX_BODY_BYTES { return Err("response_too_large".into()); }
    if bytes.iter().all(u8::is_ascii_whitespace) { return Ok(json!({})); }
    serde_json::from_slice(&bytes).map_err(|_| "NetBox API returned invalid JSON".into())
}
use std::io::Read;

fn fetch(input: &Value, path: &str, query: Option<String>, page_size: usize) -> Result<Vec<Value>, String> {
    let limit = page_size.clamp(1, MAX_PAGE_SIZE);
    let mut records = Vec::new();
    for page in 0..MAX_PAGES {
        let offset = page * limit;
        let mut params = vec![("limit", limit.to_string()), ("offset", offset.to_string())];
        if let Some(q) = &query { if !q.is_empty() { params.push(("q", q.clone())); } }
        let result = request_json(input, &format!("api/{path}"), &params)?;
        let page_records = result.get("results").and_then(Value::as_array).ok_or_else(|| "NetBox API returned malformed records".to_string())?;
        if page_records.len() > MAX_BODY_BYTES / 2 { return Err("response_too_large".into()); }
        records.extend(page_records.iter().cloned());
        if result.get("next").is_some_and(|v| !v.is_null() && !text(v).is_empty()) || page_records.len() == limit {
            if page_records.is_empty() { break; }
            continue;
        }
        break;
    }
    Ok(records)
}

pub fn fetch_endpoint_page(input: &Value) -> Result<Value,String> {
    let ep=text(&input["endpoint"]);let limit=input["page_size"].as_u64().unwrap_or(1000).clamp(1,MAX_PAGE_SIZE as u64);let offset=input["offset"].as_u64().unwrap_or(0);
    let mut params=vec![("limit",limit.to_string()),("offset",offset.to_string())];let q=text(&input["query"]);if !q.is_empty(){params.push(("q",q));}
    let fields=input["fields"].as_array().map(|a|a.iter().map(text).collect::<Vec<_>>().join(",")).unwrap_or_default();if !fields.is_empty(){params.push(("fields",fields));}
    request_json(input,&format!("api/{}",ep.trim_start_matches('/')),&params)
}
pub fn fetch_endpoint(input:&Value)->Result<Value,String>{
    let ep=text(&input["endpoint"]);let q=text(&input["query"]);let query=if q.is_empty(){None}else{Some(q.clone())};let limit=input["page_size"].as_u64().unwrap_or(1000).clamp(1,MAX_PAGE_SIZE as u64) as usize;
    let rows=fetch(input,&ep,query,limit)?;let fields=input["fields"].as_array().cloned().unwrap_or_default();Ok(json!({"endpoint":ep,"records":rows,"query":if q.is_empty(){Value::Null}else{json!(q)},"fields":if fields.is_empty(){Value::Null}else{json!(fields)},"page_size":limit}))
}

pub fn ping(input: &Value) -> Result<Value, String> {
    request_json(input, "api/", &[])?;
    Ok(json!({"connected":true}))
}

pub fn collect(input: &Value) -> Result<Value, String> {
    // Validate connectivity with the same lightweight health request used by connect().
    request_json(input, "api/", &[])?;
    let kw = input.get("kwargs").unwrap_or(&Value::Null);
    let site = kw.get("site").filter(|v| !text(v).is_empty()).or_else(|| payload(input).get("site"));
    let role = kw.get("role").filter(|v| !text(v).is_empty()).or_else(|| payload(input).get("role"));
    let mut filters = Vec::new();
    if let Some(v) = site { if !text(v).is_empty() { filters.push(format!("site={}", text(v))); } }
    if let Some(v) = role { if !text(v).is_empty() { filters.push(format!("role={}", text(v))); } }
    let query = if filters.is_empty() { None } else { Some(filters.join(" ")) };
    let page_size = setting(input, "page_size", "1000").parse::<usize>().unwrap_or(1000);
    let mut raw = json!({"source":"netbox"});
    raw["devices"] = json!(fetch(input, &endpoint(input,"device_endpoint","dcim/devices/"), query, page_size)?);
    raw["interfaces"] = json!(fetch(input, &endpoint(input,"interface_endpoint","dcim/interfaces/"), None, page_size)?);
    raw["cables"] = json!(fetch(input, &endpoint(input,"cable_endpoint","dcim/cables/"), None, page_size)?);
    raw["prefixes"] = json!(fetch(input, &endpoint(input,"prefix_endpoint","ipam/prefixes/"), None, page_size)?);
    raw["ip_addresses"] = json!(fetch(input, &endpoint(input,"ip_endpoint","ipam/ip-addresses/"), None, page_size)?);
    normalize_bundle(input, &raw)
}

pub fn normalize(input: &Value) -> Result<Value, String> {
    let raw = input.get("raw_data").unwrap_or(&Value::Null);
    normalize_bundle(input, raw)
}

fn normalize_bundle(input: &Value, raw: &Value) -> Result<Value, String> {
    let devices_raw = raw.get("devices").and_then(Value::as_array).cloned().unwrap_or_default();
    let interfaces_raw = raw.get("interfaces").and_then(Value::as_array).cloned().unwrap_or_default();
    let cables_raw = raw.get("cables").and_then(Value::as_array).cloned().unwrap_or_default();
    let prefixes_raw = raw.get("prefixes").and_then(Value::as_array).cloned().unwrap_or_default();
    let ips_raw = raw.get("ip_addresses").and_then(Value::as_array).cloned().unwrap_or_default();
    let mut assignments = std::collections::HashMap::<String, Vec<Value>>::new();
    for ip in &ips_raw {
        if !value(ip,"assigned_object_type").to_lowercase().contains("interface") { continue; }
        let id = value(ip,"assigned_object_id"); if id.is_empty() { continue; }
        assignments.entry(id).or_default().push(json!({"id":value(ip,"id"),"address":or_value(ip,"address","display"),"family":raw_value(ip,"family"),"status":raw_value(ip,"status"),"dns_name":raw_value(ip,"dns_name"),"description":raw_value(ip,"description")}));
    }
    let mut devices = Vec::new();
    for r in &devices_raw {
        let model = first(&[nested(r,&["device_type","display"]),nested(r,&["device_type","model"])]);
        let manufacturer = first(&[nested(r,&["device_type","manufacturer","display"]),nested(r,&["device_type","manufacturer","name"])]);
        let hostname = first(&[value(r,"name"),value(r,"display"),value(r,"id")]);
        let rack = first(&[nested(r,&["rack","name"]),nested(r,&["rack","display"])]);
        let role = first(&[nested(r,&["device_role","slug"]),nested(r,&["device_role","name"])]);
        let site = first(&[nested(r,&["site","slug"]),nested(r,&["site","name"])]);
        let primary_ip = first(&[nested(r,&["primary_ip4","address"]),nested(r,&["primary_ip6","address"])]);
        let serial = value(r,"serial");
        let kind_text = format!("{} {}",role,model).to_lowercase();
        let device_kind = map_device_type(&kind_text);
        let mut payload = json!({"id":value(r,"id"),"hostname":hostname,"device_type":device_kind,"vendor":manufacturer,"model":model,"role":role,"site":site,"rack":rack,"rack_position":int_opt(r,"position"),"rack_face":first(&[nested(r,&["face","value"]),value(r,"face")]),"management_ip":primary_ip,"serial_number":serial,"status":raw_value(r,"status"),"last_updated":raw_value(r,"last_updated"),"custom_fields":raw_value(r,"custom_fields")});
        payload["correlation_keys"] = correlation(&[("hostname",hostname.clone()),("management_ip",primary_ip),("serial",serial),("site",site),("rack",rack),("role",role),("vendor",manufacturer),("model",model)]);
        let meta = json!({"netbox_id":value(r,"id"),"last_updated":raw_value(r,"last_updated"),"endpoint":"dcim/devices","connector":connector_name(input,"netbox")});
        devices.push(record("device","netbox",first(&[value(r,"id"),hostname]),payload,0.94,meta));
    }
    let mut interfaces = Vec::new();
    for r in &interfaces_raw {
        let dev_id = nested(r,&["device","id"]); let dev_name = nested(r,&["device","name"]); let name = value(r,"name"); let id = value(r,"id");
        let tagged = r.get("tagged_vlans").and_then(Value::as_array).cloned().unwrap_or_else(|| if r.get("tagged_vlans").is_some_and(|v| !v.is_null()) { vec![r["tagged_vlans"].clone()] } else { vec![] });
        let mut p = json!({"id":id,"device_id":dev_id,"device_name":dev_name,"name":name,"type":raw_value(r,"type"),"enabled":bool_value(r,"enabled"),"description":raw_value(r,"description"),"label":raw_value(r,"label"),"mac_address":raw_value(r,"mac_address"),"mtu":int_opt(r,"mtu"),"lag":first(&[nested(r,&["lag","name"]),nested(r,&["lag","id"])]),"untagged_vlan":first(&[nested(r,&["untagged_vlan","vid"]),nested(r,&["untagged_vlan","name"])]),"tagged_vlans":tagged.iter().map(|v|first(&[nested(v,&["vid"]),nested(v,&["name"])] )).collect::<Vec<_>>(),"ip_addresses":assignments.get(&id).cloned().unwrap_or_default(),"last_updated":raw_value(r,"last_updated"),"custom_fields":raw_value(r,"custom_fields")});
        p["correlation_keys"] = correlation(&[("device_name",dev_name.clone()),("interface_name",name.clone()),("mac_address",value(r,"mac_address")),("label",value(r,"label")),("device_id",dev_id.clone())]);
        let meta=json!({"netbox_id":id,"device_id":dev_id,"endpoint":"dcim/interfaces","connector":connector_name(input,"netbox")});
        interfaces.push(record("interface","netbox",first(&[id,format!("{dev_id}:{name}")]),p,0.91,meta));
    }
    let mut links=Vec::new();
    for r in &cables_raw {
        let a=termination(r,"a");let b=termination(r,"b");
        let mut p=json!({"id":value(r,"id"),"status":raw_value(r,"status"),"label":raw_value(r,"label"),"type":raw_value(r,"type"),"length":raw_value(r,"length"),"length_unit":raw_value(r,"length_unit"),"termination_a":a,"termination_b":b,"last_updated":raw_value(r,"last_updated"),"custom_fields":raw_value(r,"custom_fields")});
        p["correlation_keys"]=correlation(&[("a_device",text(&p["termination_a"]["device_name"])),("a_interface",text(&p["termination_a"]["interface_name"])),("b_device",text(&p["termination_b"]["device_name"])),("b_interface",text(&p["termination_b"]["interface_name"])),("type",value(r,"type")),("label",value(r,"label"))]);
        let meta=json!({"netbox_id":value(r,"id"),"endpoint":"dcim/cables","connector":connector_name(input,"netbox")});
        links.push(record("link","netbox",first(&[value(r,"id"),format!("{}->{}",p["termination_a"]["endpoint"],p["termination_b"]["endpoint"])]),p,0.95,meta));
    }
    let mut subnets=Vec::new();
    for r in &prefixes_raw {
        let mut p=json!({"id":value(r,"id"),"prefix":value(r,"prefix"),"status":value(r,"status"),"role":first(&[nested(r,&["role","slug"]),nested(r,&["role","name"])]),"site":first(&[nested(r,&["site","slug"]),nested(r,&["site","name"])]),"tenant":first(&[nested(r,&["tenant","slug"]),nested(r,&["tenant","name"])]),"vlan":first(&[nested(r,&["vlan","vid"]),nested(r,&["vlan","name"])]),"description":value(r,"description"),"family":value(r,"family"),"is_pool":bool_value(r,"is_pool"),"last_updated":value(r,"last_updated"),"custom_fields":raw_value(r,"custom_fields")});
        let meta=json!({"netbox_id":value(r,"id"),"endpoint":"ipam/prefixes","connector":connector_name(input,"netbox")});
        subnets.push(record("subnet","netbox",first(&[value(r,"id"),value(r,"prefix")]),p,0.88,meta));
    }
    let mut addresses=Vec::new();
    for r in &ips_raw {
        let addr=value(r,"address");let assigned_type=value(r,"assigned_object_type");let assigned_id=value(r,"assigned_object_id");let assigned_name=nested(r,&["assigned_object","name"]);let device_name=nested(r,&["assigned_object","device","name"]);
        let p=json!({"id":value(r,"id"),"address":addr,"family":value(r,"family"),"status":value(r,"status"),"assigned_object_type":assigned_type,"assigned_object_id":assigned_id,"assigned_object_name":assigned_name,"device_name":device_name,"interface_name":nested(r,&["assigned_object","name"]),"dns_name":value(r,"dns_name"),"description":value(r,"description"),"last_updated":value(r,"last_updated"),"custom_fields":raw_value(r,"custom_fields")});
        let meta=json!({"netbox_id":value(r,"id"),"endpoint":"ipam/ip-addresses","connector":connector_name(input,"netbox")});
        addresses.push(record("address","netbox",first(&[value(r,"id"),addr]),p,0.86,meta));
    }
    let mut all=Vec::new();all.extend(devices.clone());all.extend(interfaces.clone());all.extend(links.clone());all.extend(subnets.clone());all.extend(addresses.clone());
    let last=latest_timestamp(all.iter().map(|r|text(&r["payload"]["last_updated"])),None);
    Ok(json!({"status":"completed","connector":connector_name(input,"netbox"),"record_type":"topology","count":all.len(),"devices":devices,"interfaces":interfaces,"links":links,"subnets":subnets,"addresses":addresses,"records":all,"sync":{"last_sync":last,"source":"netbox"}}))
}

fn connector_name(input:&Value,default:&str)->String { let name=text(&config(input)["connector_type"]);if name.is_empty(){default.to_string()}else{name} }
fn first(values:&[String])->String { values.iter().find(|x|!x.is_empty()).cloned().unwrap_or_default() }
fn or_value(r:&Value,a:&str,b:&str)->String { first(&[value(r,a),value(r,b)]) }
fn bool_value(r:&Value,k:&str)->bool { let v=r.get(k).unwrap_or(&Value::Null); match v {Value::Bool(b)=>*b,Value::String(s)=>["true","1","yes","on"].contains(&s.trim().to_lowercase().as_str()),Value::Number(n)=>n.as_i64().unwrap_or(0)!=0,_=>false} }
fn int_opt(r:&Value,k:&str)->Value { r.get(k).and_then(|v| if v.is_number(){v.as_i64()}else{text(v).parse::<i64>().ok()}).map(|n|json!(n)).unwrap_or(Value::Null) }
fn map_device_type(t:&str)->&'static str { if ["firewall","fw","security"].iter().any(|x|t.contains(x)){"firewall"}else if ["router","wan","edge"].iter().any(|x|t.contains(x)){"router"}else if ["switch","access","leaf","spine"].iter().any(|x|t.contains(x)){"switch"}else if ["server","compute","host"].iter().any(|x|t.contains(x)){"server"}else{"network_device"} }
fn termination(r:&Value,s:&str)->Value { let key=format!("termination_{s}");let n=r.get(&key).unwrap_or(&Value::Null);if n.is_object(){json!({"endpoint":first(&[value(n,"name"),value(n,"display"),value(n,"id")]),"device_id":nested(n,&["device","id"]),"device_name":nested(n,&["device","name"]),"interface_id":value(n,"id"),"interface_name":value(n,"name"),"termination_type":first(&[value(n,"_type"),value(r,&format!("termination_{s}_type"))])})}else{json!({"endpoint":value(r,&key),"device_id":first(&[value(r,&format!("termination_{s}_object_id")),value(r,&format!("termination_{s}_id"))]),"device_name":value(r,&format!("termination_{s}_object_name")),"interface_id":value(r,&format!("termination_{s}_id")),"interface_name":value(r,&key),"termination_type":value(r,&format!("termination_{s}_type"))})} }
