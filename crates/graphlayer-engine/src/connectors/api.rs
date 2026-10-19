use super::common::{normalize_base_url, MAX_BODY_BYTES};
use reqwest::{
    blocking::{Client, Response},
    header::{HeaderMap, HeaderName, HeaderValue, ACCEPT, CONTENT_TYPE},
};
use serde::Deserialize;
use serde_json::{json, Value};
use sha2::{Digest, Sha256};
use std::collections::HashMap;
use std::{
    fs,
    io::Read,
    sync::{Mutex, OnceLock},
    thread,
    time::{Duration, Instant},
};

static REQUEST_CLOCK: OnceLock<Mutex<Instant>> = OnceLock::new();
fn rate_limit() {
    let clock =
        REQUEST_CLOCK.get_or_init(|| Mutex::new(Instant::now() - Duration::from_millis(10)));
    if let Ok(mut last) = clock.lock() {
        let due = *last + Duration::from_millis(10);
        let now = Instant::now();
        if due > now {
            thread::sleep(due - now)
        }
        *last = Instant::now();
    }
}

pub struct SessionDispatcher {
    sessions: HashMap<String, Value>,
}
impl SessionDispatcher {
    pub fn new() -> Self {
        Self {
            sessions: HashMap::new(),
        }
    }
    pub fn dispatch(&mut self, op: &str, input: Value) -> Result<Value, String> {
        if op == "api.connect" {
            let id = text(&input["session_id"]);
            if id.is_empty() || id.len() > 256 {
                return Err("invalid_session_id".into());
            }
            let mut cfg = input.get("config").cloned().unwrap_or_else(|| json!({}));
            let kind = text(&input["device_type"]);
            authenticate(&mut cfg, &kind)?;
            let _ = health(&json!({"config":cfg}))?;
            self.sessions
                .insert(id.clone(), json!({"config":cfg,"device_type":kind}));
            return Ok(json!({"session_id":id,"connected":true}));
        }
        if op == "api.disconnect" {
            let id = text(&input["session_id"]);
            self.sessions.remove(&id);
            return Ok(json!({"session_id":id,"disconnected":true}));
        }
        if op == "api.collect_appid" {
            let started = Instant::now();
            let start_time = chrono::Utc::now().to_rfc3339();
            let id = text(&input["session_id"]);
            let saved = self
                .sessions
                .get(&id)
                .ok_or_else(|| "api_session_not_found".to_string())?;
            let cfg = &saved["config"];
            if text(&saved["device_type"]).to_lowercase() != "palo_alto_panorama" {
                return Err("unsupported_appid_device_type".into());
            }
            let device_id = text(&input["device_id"]);
            let mut firmware = "unknown".to_string();
            let version_query = json!({"type":"op","cmd":"<show><system><info></info></show>","key":text(&cfg["api_key"])});
            if let Ok(r) =
                request(&json!({"config":cfg,"method":"GET","path":"/api/","query":version_query}))
            {
                if let Some(xml) = r["body"].as_str() {
                    firmware =
                        super::appid::parse_panos_firmware(xml).unwrap_or_else(|| "unknown".into());
                }
            }
            let app_query = json!({"type":"op","cmd":"<show><predefined><application></application></predefined></show>","key":text(&cfg["api_key"])});
            let response =
                request(&json!({"config":cfg,"method":"GET","path":"/api/","query":app_query}))?;
            if !(200..300).contains(&response["status"].as_u64().unwrap_or(0)) {
                return Err("panos_appid_request_failed".into());
            }
            let xml = response["body"]
                .as_str()
                .ok_or_else(|| "invalid_appid_xml".to_string())?;
            let parsed = super::appid::panos_cache(xml, &device_id, &firmware)?;
            let end_time = chrono::Utc::now().to_rfc3339();
            let mut result = parsed.clone();
            result["metadata"]["firmware_version"] = json!(firmware);
            result["start_time"] = json!(start_time);
            result["end_time"] = json!(end_time);
            result["duration_ms"] = json!(started.elapsed().as_millis() as u64);
            return Ok(result);
        }
        if op.starts_with("api.") {
            let id = text(&input["session_id"]);
            let saved = self
                .sessions
                .get(&id)
                .ok_or_else(|| "api_session_not_found".to_string())?;
            let mut req = input;
            req["config"] = saved["config"].clone();
            if req.get("device_type").is_none() {
                req["device_type"] = saved["device_type"].clone()
            }
            return dispatch(op, &req);
        }
        Err("unsupported_operation".into())
    }
}

fn authenticate(cfg: &mut Value, kind: &str) -> Result<(), String> {
    match kind.to_lowercase().as_str() {
        "cisco_fmc" => {
            let result = request(
                &json!({"config":cfg,"method":"POST","path":"/api/fmc_platform/v1/auth/generatetoken"}),
            )?;
            if result["status"].as_u64() != Some(204) {
                return Err("api_authentication_failed".into());
            }
            let token = result["headers"]["x-auth-access-token"].clone();
            if token.is_null() {
                return Err("api_authentication_failed".into());
            }
            cfg["auth_token"] = token;
            Ok(())
        }
        "palo_alto_panorama" => {
            if text(&cfg["api_key"]).is_empty() {
                Err("api_key_required".into())
            } else {
                Ok(())
            }
        }
        "fortinet_fortimanager" => {
            let body = json!({"id":1,"method":"exec","params":[{"url":"/sys/login/user","data":{"user":cfg["username"],"passwd":cfg["password"]}}]});
            let r = request(&json!({"config":cfg,"method":"POST","path":"/jsonrpc","body":body}))?;
            let token = r["body"]["session"].clone();
            if token.is_null() {
                return Err("api_authentication_failed".into());
            }
            cfg["auth_token"] = token;
            Ok(())
        }
        _ => Ok(()),
    }
}

pub fn dispatch(operation: &str, input: &Value) -> Result<Value, String> {
    match operation {
        "api.request" => request(input),
        "api.health" => health(input),
        "api.device_info" => device_info(input),
        "api.config" => get_config(input),
        "api.supported_commands" => Ok(commands(&text(&input["device_type"]))),
        "api.vendor_operation" => vendor_operation(input),
        "api.inventory" => inventory(input),
        "api.collect_appid" => Err("stateful_api_operation_requires_session_mode".into()),
        _ => Err("unsupported_operation".into()),
    }
}

fn base(config: &Value) -> Result<url::Url, String> {
    let host = text(&config["host"]);
    if host.is_empty() {
        return Err("missing_host".into());
    }
    let port = config["port"]
        .as_u64()
        .filter(|p| *p > 0 && *p <= 65535)
        .unwrap_or(443);
    let scheme = if port == 80 { "http" } else { "https" };
    let target = if host.contains("://") {
        host
    } else {
        format!("{scheme}://{host}:{port}")
    };
    normalize_base_url(&json!(target), "missing_host")
}
fn request(input: &Value) -> Result<Value, String> {
    let config = input
        .get("config")
        .or_else(|| input.get("connector_data"))
        .unwrap_or(input);
    let base = base(config)?;
    let path = text(&input["path"]);
    if path.is_empty() || path.len() > 4096 || path.contains('\\') {
        return Err("invalid_endpoint".into());
    }
    let url = base
        .join(path.trim_start_matches('/'))
        .map_err(|_| "invalid_endpoint".to_string())?;
    if url.scheme() != base.scheme()
        || url.host_str() != base.host_str()
        || url.port_or_known_default() != base.port_or_known_default()
    {
        return Err("invalid_endpoint".into());
    }
    let timeout = config["timeout_seconds"]
        .as_u64()
        .unwrap_or(30)
        .clamp(1, 120);
    let mut builder = Client::builder()
        .timeout(Duration::from_secs(timeout))
        .redirect(reqwest::redirect::Policy::none());
    if let Some(ca) = config
        .get("ca_cert")
        .and_then(Value::as_str)
        .filter(|s| !s.is_empty())
    {
        let pem = fs::read(ca).map_err(|_| "ca_certificate_unavailable".to_string())?;
        let cert = reqwest::Certificate::from_pem(&pem)
            .map_err(|_| "invalid_ca_certificate".to_string())?;
        builder = builder.add_root_certificate(cert);
    }
    if let (Some(cert), Some(key)) = (
        config.get("cert_file").and_then(Value::as_str),
        config.get("key_file_api").and_then(Value::as_str),
    ) {
        if !cert.is_empty() && !key.is_empty() {
            let mut pem =
                fs::read(cert).map_err(|_| "client_certificate_unavailable".to_string())?;
            pem.extend_from_slice(
                &fs::read(key).map_err(|_| "client_key_unavailable".to_string())?,
            );
            builder = builder.identity(
                reqwest::Identity::from_pem(&pem)
                    .map_err(|_| "invalid_client_identity".to_string())?,
            );
        }
    }
    let client = builder
        .build()
        .map_err(|_| "http_client_error".to_string())?;
    let method = text(&input["method"]).to_uppercase();
    if !matches!(
        method.as_str(),
        "GET" | "POST" | "PUT" | "PATCH" | "DELETE" | "HEAD"
    ) {
        return Err("unsupported_http_method".into());
    }
    let mut request = client
        .request(
            method
                .parse()
                .map_err(|_| "unsupported_http_method".to_string())?,
            url,
        )
        .header(ACCEPT, "application/json");
    let mut headers = HeaderMap::new();
    if let Some(map) = config["headers"].as_object() {
        if map.len() > 64 {
            return Err("too_many_headers".into());
        }
        let mut header_bytes = 0usize;
        for (k, v) in map {
            let raw = text(v);
            header_bytes = header_bytes
                .saturating_add(k.len())
                .saturating_add(raw.len());
            if k.len() > 256 || raw.len() > 8192 || header_bytes > 65536 {
                return Err("headers_too_large".into());
            }
            let name =
                HeaderName::from_bytes(k.as_bytes()).map_err(|_| "invalid_header".to_string())?;
            let value = HeaderValue::from_str(&raw).map_err(|_| "invalid_header".to_string())?;
            headers.insert(name, value);
        }
    }
    request = request.headers(headers);
    let key = text(&config["api_key"]);
    let token = text(&config["token"]);
    let username = text(&config["username"]);
    let password = text(&config["password"]);
    let session_token = text(&config["auth_token"]);
    if !session_token.is_empty() {
        request = request.header("X-auth-access-token", session_token)
    } else if !key.is_empty() {
        request = request.header("X-API-Key", &key).header("X-PAN-KEY", key)
    } else if !token.is_empty() {
        request = request.bearer_auth(token)
    } else if !username.is_empty() {
        request = request.basic_auth(username, Some(password))
    }
    if let Some(query) = input.get("query").and_then(Value::as_object) {
        if query.len() > 128 {
            return Err("too_many_query_parameters".into());
        }
        let pairs = query
            .iter()
            .map(|(k, v)| (k.as_str(), text(v)))
            .collect::<Vec<_>>();
        if pairs.iter().any(|(k, v)| k.len() > 256 || v.len() > 8192) {
            return Err("query_parameters_too_large".into());
        }
        request = request.query(&pairs)
    }
    let body = input.get("body").filter(|v| !v.is_null());
    if let Some(body) = body {
        if serde_json::to_vec(body)
            .map_err(|_| "request_body_serialize_failed".to_string())?
            .len()
            > MAX_BODY_BYTES
        {
            return Err("request_body_too_large".into());
        }
        request = request.header(CONTENT_TYPE, "application/json").json(body)
    }
    let retries = config["retry_count"].as_u64().unwrap_or(3).clamp(1, 8);
    let delay = config["retry_delay_ms"]
        .as_u64()
        .unwrap_or(250)
        .clamp(0, 5000);
    let mut last_error = "request_failed".to_string();
    for attempt in 0..retries {
        rate_limit();
        let result = request
            .try_clone()
            .ok_or_else(|| "request_not_replayable".to_string())?
            .send();
        match result {
            Ok(response) if response.status().as_u16() < 500 => return response_value(response),
            Ok(response) => {
                last_error = format!("http_status_{}", response.status().as_u16());
                if attempt + 1 < retries {
                    thread::sleep(Duration::from_millis(delay.saturating_mul(attempt + 1)));
                }
            }
            Err(_) => {
                last_error = "request_failed".into();
                if attempt + 1 < retries {
                    thread::sleep(Duration::from_millis(delay.saturating_mul(attempt + 1)));
                }
            }
        }
    }
    Err(last_error)
}
fn response_value(response: Response) -> Result<Value, String> {
    let status = response.status().as_u16();
    let mut headers = serde_json::Map::new();
    for (k, v) in response.headers() {
        if let Ok(s) = v.to_str() {
            headers.insert(k.to_string(), json!(s));
        }
    }
    if response
        .content_length()
        .is_some_and(|n| n > MAX_BODY_BYTES as u64)
    {
        return Err("response_too_large".into());
    }
    let mut bytes = Vec::new();
    response
        .take((MAX_BODY_BYTES + 1) as u64)
        .read_to_end(&mut bytes)
        .map_err(|_| "response_read_failed".to_string())?;
    if bytes.len() > MAX_BODY_BYTES {
        return Err("response_too_large".into());
    }
    let body = serde_json::from_slice::<Value>(&bytes)
        .unwrap_or_else(|_| json!(String::from_utf8_lossy(&bytes).to_string()));
    let output = body
        .as_str()
        .map(str::to_string)
        .unwrap_or_else(|| serde_json::to_string(&body).unwrap_or_default());
    Ok(json!({"status":status,"headers":headers,"body":body,"output":output,"data":body}))
}
fn health(i: &Value) -> Result<Value, String> {
    let c = i.get("config").unwrap_or(i);
    let paths = if let Some(p) = c
        .get("health_endpoint")
        .and_then(Value::as_str)
        .filter(|s| !s.is_empty())
    {
        vec![p.to_string()]
    } else {
        vec![
            "/api/health".into(),
            "/api/version".into(),
            "/health".into(),
            "/version".into(),
            "/".into(),
        ]
    };
    let mut last = 0;
    for path in paths {
        if let Ok(result) = request(&json!({"config":c,"method":"GET","path":path})) {
            last = result["status"].as_u64().unwrap_or(0);
            if (200..300).contains(&last) {
                return Ok(json!({"connected":true,"status":last}));
            }
        }
    }
    Err(format!("no_accessible_api_endpoints_{last}"))
}
fn device_info(i: &Value) -> Result<Value, String> {
    let c = i.get("config").unwrap_or(i);
    let host = text(&c["host"]);
    let kind = text(&i["device_type"]);
    let (vendor, model) = vendor_model(&kind);
    let os = match kind.to_lowercase().as_str() {
        "cisco_fmc" => "firepower",
        "palo_alto_panorama" => "panos",
        "fortinet_fortimanager" => "fortios",
        "solarwinds_ncm" => "solarwinds",
        _ => "unknown",
    };
    Ok(
        json!({"id":format!("api_{host}_{}",c["port"].as_u64().unwrap_or(443)),"hostname":host,"ip_address":host,"vendor":vendor,"model":model,"os_type":os,"last_seen":chrono::Utc::now().to_rfc3339(),"connection_type":"api","capabilities":capabilities(&kind)}),
    )
}
fn vendor_model(kind: &str) -> (&'static str, &'static str) {
    match kind.to_lowercase().as_str() {
        "cisco_fmc" => ("Cisco", "Firepower Management Center"),
        "palo_alto_panorama" => ("Palo Alto Networks", "Panorama"),
        "fortinet_fortimanager" => ("Fortinet", "FortiManager"),
        "solarwinds_ncm" => ("SolarWinds", "Network Configuration Manager"),
        _ => ("Unknown", "Generic API"),
    }
}
fn commands(kind: &str) -> Value {
    let list = match kind.to_lowercase().as_str() {
        "cisco_fmc" => vec![
            "get_device_list",
            "get_device_config",
            "get_security_events",
            "get_policies",
        ],
        "palo_alto_panorama" => vec![
            "get_device_list",
            "get_device_config",
            "get_device_groups",
            "get_templates",
        ],
        "solarwinds_ncm" => vec![
            "get_device_list",
            "get_device_config",
            "get_node_info",
            "execute_script",
        ],
        "fortinet_fortimanager" => vec!["get_device_list"],
        _ => vec!["get_device_list", "get_device_config"],
    };
    json!(list)
}
fn capabilities(kind: &str) -> Value {
    let mut c = vec![
        "api_management",
        "device_inventory",
        "configuration_management",
    ];
    if kind.to_lowercase().contains("ncm") {
        c.push("network_config_management");
    }
    json!(c)
}
fn vendor_operation(i: &Value) -> Result<Value, String> {
    let c = i.get("config").unwrap_or(i);
    let kind = text(&i["device_type"]).to_lowercase();
    let op = text(&i["operation"]);
    let (method, path, query, body) = match (kind.as_str(), op.as_str()) {
        ("cisco_fmc", "get_device_list") => (
            "GET",
            "/api/fmc_config/v1/domain/default/devices/devicerecords",
            Value::Null,
            Value::Null,
        ),
        ("cisco_fmc", "get_security_events") => (
            "GET",
            "/api/fmc_config/v1/domain/default/policy/accesspolicies",
            Value::Null,
            Value::Null,
        ),
        ("cisco_fmc", "get_policies") => (
            "GET",
            "/api/fmc_config/v1/domain/default/policy/accesspolicies",
            Value::Null,
            Value::Null,
        ),
        ("palo_alto_panorama", "get_device_list") => (
            "GET",
            "/api/",
            json!({"type":"op","cmd":"<show><devices><connected></connected></devices></show>"}),
            Value::Null,
        ),
        ("palo_alto_panorama", "get_device_groups") => (
            "GET",
            "/api/",
            json!({"type":"config","action":"get","xpath":"/config/devices/entry[@name='localhost.localdomain']/device-group"}),
            Value::Null,
        ),
        ("palo_alto_panorama", "get_templates") => (
            "GET",
            "/api/",
            json!({"type":"config","action":"get","xpath":"/config/devices/entry[@name='localhost.localdomain']/template"}),
            Value::Null,
        ),
        ("palo_alto_panorama", "get_config") => (
            "GET",
            "/api/",
            json!({"type":"op","cmd":"<show><config><running/></config></show>"}),
            Value::Null,
        ),
        ("solarwinds_ncm", "get_device_list") => (
            "POST",
            "/SolarWinds/InformationService/v3/Json/Query",
            Value::Null,
            json!({"query":"SELECT NodeID, IPAddress, Caption, Vendor, MachineType, Status FROM Orion.Nodes"}),
        ),
        ("solarwinds_ncm", "execute_script") => (
            "POST",
            "/SolarWinds/InformationService/v3/Json/Invoke/Orion.Nodes/ExecuteScript",
            Value::Null,
            json!({"script":text(&i["script"])}),
        ),
        ("fortinet_fortimanager", "get_device_list") => (
            "POST",
            "/jsonrpc",
            Value::Null,
            json!({"id":1,"method":"get","params":[{"url":"/dvmdb/adom/root/device"}]}),
        ),
        ("fortinet_fortimanager", "get_device_info") => (
            "POST",
            "/jsonrpc",
            Value::Null,
            json!({"id":1,"method":"get","params":[{"url":"/dvmdb/adom/root/device"}]}),
        ),
        _ => return Err("unsupported_vendor_operation".into()),
    };
    request(&json!({"config":c,"method":method,"path":path,"query":query,"body":body}))
}
fn inventory(i: &Value) -> Result<Value, String> {
    let r = vendor_operation(
        &json!({"config":i.get("config").unwrap_or(i),"device_type":i["device_type"],"operation":"get_device_list"}),
    )?;
    let b = &r["body"];
    let kind = text(&i["device_type"]).to_lowercase();
    let now = chrono::Utc::now().to_rfc3339();
    let rows = if kind == "cisco_fmc" {
        b["items"].as_array().cloned().unwrap_or_default().into_iter().map(|v|json!({"id":v["id"],"hostname":v["hostName"],"ip_address":v["hostName"],"model":v["model"],"vendor":"Cisco","os_type":"firepower","last_seen":now,"connection_type":"api"})).collect::<Vec<_>>()
    } else if kind == "solarwinds_ncm" {
        b["results"].as_array().cloned().unwrap_or_default().into_iter().map(|v|json!({"id":v["NodeID"].as_str().map(str::to_string).or_else(||v["NodeID"].as_i64().map(|n|n.to_string())).unwrap_or_default(),"ip_address":v["IPAddress"],"hostname":v["Caption"],"model":v["MachineType"],"vendor":v["Vendor"],"os_type":v["MachineType"],"last_seen":now,"connection_type":"api"})).collect()
    } else if kind == "fortinet_fortimanager" {
        b["result"].as_array().and_then(|a|a.first()).and_then(|v|v["data"].as_array()).cloned().unwrap_or_default().into_iter().map(|v|json!({"id":v["name"],"ip_address":v["ip"],"hostname":v["name"],"model":v["platform_str"],"vendor":"Fortinet","os_type":"fortios","last_seen":now,"connection_type":"api"})).collect()
    } else if kind == "palo_alto_panorama" {
        let xml = b
            .as_str()
            .ok_or_else(|| "invalid_panorama_xml_response".to_string())?;
        let response: PanoramaResponse = quick_xml::de::from_str(xml)
            .map_err(|_| "invalid_panorama_xml_response".to_string())?;
        if response.status != "success" {
            return Err("panorama_inventory_failed".into());
        }
        response.result.devices.entry.into_iter().map(|v|json!({"id":v.serial,"hostname":if v.hostname.is_empty(){v.name}else{v.hostname},"model":v.model,"vendor":"Palo Alto Networks","os_type":"panos","last_seen":now,"connection_type":"api"})).collect()
    } else {
        return Err("unsupported_vendor_operation".into());
    };
    Ok(json!(rows))
}
fn get_config(i: &Value) -> Result<Value, String> {
    let started = Instant::now();
    let c = i.get("config").unwrap_or(i);
    let kind = text(&i["device_type"]).to_lowercase();
    let config_type = text(&i["config_type"]);
    let (method, path, query, body) = match kind.as_str() {
        "cisco_fmc" => (
            "GET",
            "/api/fmc_config/v1/domain/default/policy/accesspolicies",
            Value::Null,
            Value::Null,
        ),
        "palo_alto_panorama" => {
            let tag = if config_type == "candidate" {
                "candidate"
            } else {
                "running"
            };
            (
                "GET",
                "/api/",
                json!({"type":"op","cmd":format!("<show><config><{tag}/></config></show>")}),
                Value::Null,
            )
        }
        "fortinet_fortimanager" => (
            "POST",
            "/jsonrpc",
            Value::Null,
            json!({"method":"get","params":[{"url":"/pm/config/global/obj/firewall/policy"}],"id":1,"session":c["auth_token"]}),
        ),
        "solarwinds_ncm" => {
            let escaped = config_type.replace('\'', "''");
            (
                "POST",
                "/SolarWinds/InformationService/v3/Json/Query",
                Value::Null,
                json!({"query":format!("SELECT NodeID, NodeName, ConfigDate, Config FROM NCM.NodeConfigs WHERE ConfigType = '{escaped}' ORDER BY ConfigDate DESC")}),
            )
        }
        _ => return Err("unsupported_config_type".into()),
    };
    let response =
        request(&json!({"config":c,"method":method,"path":path,"query":query,"body":body}))?;
    let data = &response["body"];
    let config_data = if let Some(s) = data.as_str() {
        s.to_string()
    } else {
        serde_json::to_string(data).unwrap_or_default()
    };
    let hash = format!("sha256:{:x}", Sha256::digest(config_data.as_bytes()));
    let host = text(&c["host"]);
    let device_id = if text(&i["device_id"]).is_empty() {
        format!("api_{host}_{}", c["port"].as_u64().unwrap_or(443))
    } else {
        text(&i["device_id"])
    };
    Ok(
        json!({"device_id":device_id,"config_type":config_type,"config_data":config_data,"hash":hash,"retrieved_at":chrono::Utc::now().to_rfc3339(),"duration":started.elapsed().as_nanos().min(u64::MAX as u128)as u64}),
    )
}
#[derive(Deserialize)]
struct PanoramaResponse {
    #[serde(rename = "@status")]
    status: String,
    result: PanoramaResult,
}
#[derive(Deserialize)]
struct PanoramaResult {
    devices: PanoramaDevices,
}
#[derive(Deserialize)]
struct PanoramaDevices {
    #[serde(rename = "entry", default)]
    entry: Vec<PanoramaDevice>,
}
#[derive(Deserialize)]
struct PanoramaDevice {
    #[serde(rename = "@name", default)]
    name: String,
    #[serde(default)]
    serial: String,
    #[serde(default)]
    hostname: String,
    #[serde(default)]
    model: String,
}
fn text(v: &Value) -> String {
    match v {
        Value::String(s) => s.clone(),
        Value::Number(n) => n.to_string(),
        Value::Bool(b) => b.to_string(),
        _ => String::new(),
    }
}
