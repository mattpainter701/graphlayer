use super::{
    common::{normalize_base_url, MAX_BODY_BYTES},
    fileshare::{create_private_dir_all, secure_write},
};
use reqwest::blocking::Client;
use serde_json::{json, Value};
use std::{io::Read, path::PathBuf, time::Duration};

const DEVICES: &str = "SELECT N.NodeID, N.Caption, N.DNS, N.IPAddress, N.Vendor, N.MachineType, CN.NodeID AS CirrusNodeID FROM Orion.Nodes N INNER JOIN Cirrus.Nodes CN ON N.NodeID = CN.CoreNodeID WHERE N.ObjectSubType = 'SNMP'";

pub fn dispatch(operation: &str, input: &Value) -> Result<Value, String> {
    match operation {
        "solarwinds.query" => query(input),
        "solarwinds.ping" => ping(input),
        "solarwinds.collect" => collect(input),
        "solarwinds.trigger" => trigger(input),
        "solarwinds.devices" => devices(input),
        "solarwinds.config" => config(input),
        "solarwinds.save_config" => save_config(input),
        "solarwinds.sanitize_filename" => Ok(json!(sanitize(&text(&input["name"])))),
        _ => Err("unsupported_operation".into()),
    }
}

fn client(input: &Value) -> Result<(Client, String, String), String> {
    let data = input.get("connector_data").unwrap_or(input);
    let payload = data.get("payload").unwrap_or(&Value::Null);
    let host = text(&data["host"]);
    let scheme = if payload
        .get("use_https")
        .and_then(Value::as_bool)
        .unwrap_or(true)
    {
        "https"
    } else {
        "http"
    };
    let target = if host.contains("://") {
        host
    } else {
        format!("{scheme}://{host}:17778")
    };
    let mut url = normalize_base_url(&json!(target), "missing_host")?;
    url.set_path("/SolarWinds/InformationService/v3/Json/Query");
    url.set_query(None);
    let verify = payload
        .get("verify_ssl")
        .and_then(Value::as_bool)
        .unwrap_or(true);
    let c = Client::builder()
        .timeout(Duration::from_secs(30))
        .danger_accept_invalid_certs(!verify)
        .redirect(reqwest::redirect::Policy::none())
        .build()
        .map_err(|_| "http_client_error".to_string())?;
    let credentials = data.get("credentials").unwrap_or(&Value::Null);
    let username = if text(&data["username"]).is_empty() {
        text(&credentials["username"])
    } else {
        text(&data["username"])
    };
    Ok((c, url.to_string(), username))
}
fn query(input: &Value) -> Result<Value, String> {
    let (client, url, user) = client(input)?;
    let data = input.get("connector_data").unwrap_or(input);
    let credentials = data.get("credentials").unwrap_or(&Value::Null);
    let password = if text(&data["password"]).is_empty() {
        text(&credentials["password"])
    } else {
        text(&data["password"])
    };
    let q = text(&input["query"]);
    if q.is_empty() || q.len() > 256 * 1024 {
        return Err("invalid_query".into());
    }
    let response = client
        .post(url)
        .basic_auth(user, Some(password))
        .json(&json!({"query":q}))
        .send()
        .map_err(|_| "request_failed".to_string())?;
    let status = response.status();
    if !status.is_success() {
        return Err(format!("http_status_{}", status.as_u16()));
    }
    let mut body = Vec::new();
    response
        .take(MAX_BODY_BYTES as u64 + 1)
        .read_to_end(&mut body)
        .map_err(|_| "response_read_failed".to_string())?;
    if body.len() > MAX_BODY_BYTES {
        return Err("response_too_large".into());
    }
    serde_json::from_slice(&body).map_err(|_| "invalid_json_response".into())
}
fn trigger(input: &Value) -> Result<Value, String> {
    let (client, url, user) = client(input)?;
    let data = input.get("connector_data").unwrap_or(input);
    let credentials = data.get("credentials").unwrap_or(&Value::Null);
    let password = if text(&data["password"]).is_empty() {
        text(&credentials["password"])
    } else {
        text(&data["password"])
    };
    let endpoint = url.replace(
        "/Json/Query",
        "/Json/Invoke/Cirrus.ConfigArchive/DownloadConfig",
    );
    let ids = input
        .get("node_ids")
        .and_then(Value::as_array)
        .cloned()
        .unwrap_or_default();
    if ids.is_empty() || ids.len() > 10000 {
        return Err("invalid_node_ids".into());
    }
    let config_type = text(&input["config_type"]);
    if config_type.is_empty() || config_type.len() > 128 {
        return Err("invalid_config_type".into());
    }
    let response = client
        .post(endpoint)
        .basic_auth(user, Some(password))
        .json(&json!([ids, config_type]))
        .send()
        .map_err(|_| "request_failed".to_string())?;
    if !response.status().is_success() {
        return Err(format!("http_status_{}", response.status().as_u16()));
    }
    let mut body = Vec::new();
    response
        .take(MAX_BODY_BYTES as u64 + 1)
        .read_to_end(&mut body)
        .map_err(|_| "response_read_failed".to_string())?;
    if body.len() > MAX_BODY_BYTES {
        return Err("response_too_large".into());
    }
    serde_json::from_slice(&body).map_err(|_| "invalid_json_response".into())
}
fn ping(input: &Value) -> Result<Value, String> {
    let response = query(
        &json!({"connector_data":input.get("connector_data").unwrap_or(input),"query":"SELECT TOP 1 EngineID FROM Orion.Engines"}),
    )?;
    let results = response
        .get("results")
        .and_then(Value::as_array)
        .cloned()
        .unwrap_or_default();
    Ok(
        json!({"status":"success","message":format!("Connected successfully. Found {} engines.",results.len()),"engine_count":results.len()}),
    )
}
fn devices(input: &Value) -> Result<Value, String> {
    let data = input.get("connector_data").unwrap_or(input);
    let mut q = DEVICES.to_string();
    let selected = data
        .get("devices")
        .or_else(|| data.get("payload").and_then(|p| p.get("devices")))
        .and_then(Value::as_array)
        .cloned()
        .unwrap_or_default();
    if !selected.is_empty() {
        let names = selected
            .iter()
            .map(|v| format!("'{}'", text(v).replace('\'', "''")))
            .collect::<Vec<_>>()
            .join(", ");
        q.push_str(&format!(" AND N.Caption IN ({names})"));
    }
    query(&json!({"connector_data":data,"query":q}))
}
fn config(input: &Value) -> Result<Value, String> {
    let data = input.get("connector_data").unwrap_or(input);
    let device = input.get("device").unwrap_or(&Value::Null);
    let node = device
        .get("CirrusNodeID")
        .and_then(Value::as_i64)
        .ok_or_else(|| "missing_cirrus_node_id".to_string())?;
    let typ = text(&input["config_type"]);
    if typ.is_empty() || typ.len() > 128 {
        return Err("invalid_config_type".into());
    }
    let q=format!("SELECT TOP 1 ConfigID, NodeID, ConfigType, Config, DownloadTime FROM Cirrus.ConfigArchive WHERE NodeID = {node} AND ConfigType = '{}' ORDER BY DownloadTime DESC",typ.replace('\'',"''"));
    let rows = query(&json!({"connector_data":data,"query":q}))?
        .get("results")
        .and_then(Value::as_array)
        .cloned()
        .unwrap_or_default();
    Ok(json!({"config":rows.first().and_then(|r|r.get("Config")).cloned().unwrap_or(Value::Null)}))
}
fn save_config(input: &Value) -> Result<Value, String> {
    let device = input.get("device").unwrap_or(&Value::Null);
    let root = PathBuf::from(text(&input["working_folder"]));
    if root.as_os_str().is_empty() {
        return Err("missing_working_folder".into());
    }
    create_private_dir_all(&root).map_err(|_| "output_directory_error".to_string())?;
    let host = text(&device["Caption"]);
    let typ = text(&input["config_type"]);
    let config = text(&input["config"]);
    if config.len() > MAX_BODY_BYTES {
        return Err("config_too_large".into());
    }
    let filename = format!("{}_{}.cfg", sanitize(&host), typ.to_lowercase());
    let output=format!("! Configuration downloaded from SolarWinds NCM\n! Device: {host}\n! IP Address: {}\n! Vendor: {}\n! Machine Type: {}\n! Config Type: {typ}\n! Downloaded: {}\n!\n{}",display(&device["IPAddress"],"N/A"),display(&device["Vendor"],"N/A"),display(&device["MachineType"],"N/A"),chrono::Utc::now().to_rfc3339(),config);
    secure_write(&root, &filename, output.as_bytes())?;
    let path = root.join(filename);
    Ok(json!({"path":path}))
}
fn collect(input: &Value) -> Result<Value, String> {
    let data = input.get("connector_data").unwrap_or(input);
    let payload = data.get("payload").unwrap_or(&Value::Null);
    let selected = payload
        .get("devices")
        .or_else(|| data.get("devices"))
        .and_then(Value::as_array)
        .cloned()
        .unwrap_or_default();
    let mut q = DEVICES.to_string();
    if !selected.is_empty() {
        let names = selected
            .iter()
            .map(|v| format!("'{}'", text(v).replace('\'', "''")))
            .collect::<Vec<_>>()
            .join(", ");
        q.push_str(&format!(" AND N.Caption IN ({names})"));
    }
    let devices = query(&json!({"connector_data":data,"query":q}))?
        .get("results")
        .and_then(Value::as_array)
        .cloned()
        .unwrap_or_default();
    let types = payload
        .get("config_types")
        .and_then(Value::as_array)
        .cloned()
        .unwrap_or_else(|| vec![json!("Running")]);
    let root = PathBuf::from(text(&input["working_folder"]));
    if root.as_os_str().is_empty() {
        return Err("missing_working_folder".into());
    }
    create_private_dir_all(&root).map_err(|_| "output_directory_error".to_string())?;
    let mut downloaded = 0usize;
    for device in &devices {
        let node = device
            .get("CirrusNodeID")
            .and_then(Value::as_i64)
            .unwrap_or(0);
        if node <= 0 {
            continue;
        }
        for typ in &types {
            let config_type = text(typ);
            if config_type.is_empty() || config_type.len() > 128 {
                continue;
            }
            let swql=format!("SELECT TOP 1 ConfigID, NodeID, ConfigType, Config, DownloadTime FROM Cirrus.ConfigArchive WHERE NodeID = {node} AND ConfigType = '{}' ORDER BY DownloadTime DESC",config_type.replace('\'',"''"));
            let rows = query(&json!({"connector_data":data,"query":swql}))?
                .get("results")
                .and_then(Value::as_array)
                .cloned()
                .unwrap_or_default();
            let Some(config) = rows
                .first()
                .and_then(|r| r.get("Config"))
                .and_then(Value::as_str)
            else {
                continue;
            };
            if config.len() > MAX_BODY_BYTES {
                return Err("config_too_large".into());
            }
            let caption = text(&device["Caption"]);
            let file = format!("{}_{}.cfg", sanitize(&caption), config_type.to_lowercase());
            let content=format!("! Configuration downloaded from SolarWinds NCM\n! Device: {caption}\n! IP Address: {}\n! Vendor: {}\n! Machine Type: {}\n! Config Type: {config_type}\n! Downloaded: {}\n!\n{}",display(&device["IPAddress"],"N/A"),display(&device["Vendor"],"N/A"),display(&device["MachineType"],"N/A"),chrono::Utc::now().to_rfc3339(),config);
            secure_write(&root, &file, content.as_bytes())?;
            downloaded += 1;
        }
    }
    Ok(
        json!({"status":"completed","devices_processed":devices.len(),"configs_downloaded":downloaded,"message":format!("Downloaded {downloaded} configs from {} devices",devices.len())}),
    )
}
fn sanitize(v: &str) -> String {
    v.replace(' ', "_")
        .chars()
        .filter(|c| !"<>:\"/\\|?*".contains(*c))
        .take(200)
        .collect()
}
fn display(v: &Value, fallback: &str) -> String {
    if v.is_null() {
        fallback.into()
    } else {
        text(v)
    }
}
fn text(v: &Value) -> String {
    match v {
        Value::String(s) => s.clone(),
        Value::Number(n) => n.to_string(),
        Value::Bool(b) => b.to_string(),
        _ => String::new(),
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn save_input(root: &std::path::Path) -> Value {
        json!({
            "working_folder": root.to_string_lossy(),
            "config_type": "Running",
            "config": "hostname edge\nusername admin secret sanitized-canary",
            "device": {
                "Caption": "edge",
                "IPAddress": "192.0.2.10",
                "Vendor": "Cisco",
                "MachineType": "ASA"
            }
        })
    }

    #[cfg(unix)]
    #[test]
    fn saved_raw_configuration_uses_private_directory_and_file_modes() {
        use std::os::unix::fs::PermissionsExt;

        let root = std::env::temp_dir().join(format!(
            "graphlayer-solarwinds-private-{}-{}",
            std::process::id(),
            std::time::SystemTime::now()
                .duration_since(std::time::UNIX_EPOCH)
                .unwrap()
                .as_nanos()
        ));
        let result = save_config(&save_input(&root)).unwrap();
        let output_path = root.join("edge_running.cfg");
        assert_eq!(result["path"].as_str(), output_path.to_str());
        assert_eq!(
            std::fs::metadata(&root).unwrap().permissions().mode() & 0o777,
            0o700
        );
        assert_eq!(
            std::fs::metadata(root.join("edge_running.cfg"))
                .unwrap()
                .permissions()
                .mode()
                & 0o777,
            0o600
        );
        let _ = std::fs::remove_dir_all(root);
    }

    #[cfg(unix)]
    #[test]
    fn saved_raw_configuration_rejects_existing_symlink_target() {
        use std::os::unix::fs::symlink;

        let root = std::env::temp_dir().join(format!(
            "graphlayer-solarwinds-link-{}-{}",
            std::process::id(),
            std::time::SystemTime::now()
                .duration_since(std::time::UNIX_EPOCH)
                .unwrap()
                .as_nanos()
        ));
        let outside = root.with_extension("outside");
        std::fs::create_dir_all(&root).unwrap();
        std::fs::write(&outside, b"preserve this file").unwrap();
        symlink(&outside, root.join("edge_running.cfg")).unwrap();

        assert_eq!(
            save_config(&save_input(&root)).unwrap_err(),
            "unsafe_output_path"
        );
        assert_eq!(std::fs::read(&outside).unwrap(), b"preserve this file");
        let _ = std::fs::remove_dir_all(root);
        let _ = std::fs::remove_file(outside);
    }
}
