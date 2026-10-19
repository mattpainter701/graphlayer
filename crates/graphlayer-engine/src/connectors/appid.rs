use chrono::Utc;
use serde::Deserialize;
use serde_json::{json, Value};
use std::collections::BTreeMap;

#[derive(Deserialize)]
struct PanResponse {
    #[serde(rename = "@status", default)]
    status: String,
    result: PanResult,
}
#[derive(Deserialize)]
struct PanResult {
    application: PanApplications,
}
#[derive(Deserialize)]
struct PanApplications {
    #[serde(default)]
    version: String,
    #[serde(rename = "entry", default)]
    entries: Vec<PanEntry>,
}
#[derive(Deserialize)]
struct PanEntry {
    #[serde(default)]
    name: String,
    #[serde(default)]
    category: String,
    #[serde(default)]
    subcategory: String,
    #[serde(default)]
    technology: String,
    #[serde(default)]
    description: String,
    #[serde(default)]
    risk: i64,
    #[serde(default)]
    deprecated: String,
    default: Option<PanDefault>,
    #[serde(rename = "use-applications")]
    use_applications: Option<PanUseApps>,
}
#[derive(Deserialize)]
struct PanDefault {
    #[serde(rename = "ident-by-ip-protocol", default)]
    protocol: String,
    #[serde(default)]
    port: String,
}
#[derive(Deserialize)]
struct PanUseApps {
    #[serde(rename = "member", default)]
    members: Vec<String>,
}

pub fn panos_cache(xml: &str, device_id: &str, firmware: &str) -> Result<Value, String> {
    if xml.len() > super::common::MAX_BODY_BYTES {
        return Err("appid_response_too_large".into());
    }
    let parsed: PanResponse =
        quick_xml::de::from_str(xml).map_err(|_| "invalid_appid_xml".to_string())?;
    if parsed.status != "success" {
        return Err("panos_appid_request_failed".into());
    }
    let now = Utc::now().to_rfc3339();
    let mut apps = BTreeMap::new();
    for entry in parsed.result.application.entries {
        let (protocol, ports) = default_service(
            entry
                .default
                .as_ref()
                .map(|d| d.protocol.as_str())
                .unwrap_or(""),
            entry
                .default
                .as_ref()
                .map(|d| d.port.as_str())
                .unwrap_or(""),
        );
        let parent = entry
            .use_applications
            .and_then(|u| u.members.into_iter().next())
            .unwrap_or_default();
        let key = entry.name.to_lowercase();
        apps.insert(key, json!({"name":entry.name,"category":entry.category,"subcategory":entry.subcategory,
            "technology":entry.technology,"description":entry.description,"risk":entry.risk,
            "default_ports":ports,"default_protocol":protocol,"used_by_malware":false,"file_type_ident":false,
            "virus_ident":false,"data_ident":false,"deprecated":entry.deprecated=="yes","parent_app":parent}));
    }
    let count = apps.len();
    let version = parsed.result.application.version;
    let status = if count >= 1000 {
        "full_success"
    } else {
        "partial_incomplete"
    };
    Ok(
        json!({"status":status,"apps_collected":count,"errors":[],"cache":{"applications":apps,"version":version.clone(),"last_updated":now,"source":"panos-xml-api"},
       "metadata":{"device_id":device_id,"device_type":"panos","firmware_version":firmware,"content_version":version,"source":"panos-xml-api","status":status,"timestamp":now,"actual_total":count,"completeness":if count>=1000{1.0}else{0.5}}}),
    )
}

pub fn default_service(ident: &str, port_field: &str) -> (String, Vec<String>) {
    let protocol = match ident.to_ascii_lowercase().as_str() {
        "udp" => "udp",
        "icmp" => "icmp",
        "icmp6" => "icmp6",
        _ => "tcp",
    }
    .to_string();
    let ports = port_field
        .split(',')
        .filter_map(|raw| {
            let value = raw.trim();
            let part = value
                .split_once('/')
                .map(|(_, suffix)| suffix)
                .unwrap_or(value)
                .trim();
            if part.is_empty() || part == "0" {
                None
            } else {
                Some(part.to_string())
            }
        })
        .collect();
    (protocol, ports)
}

pub fn parse_isdb_entry(id: i64, output: &str) -> Option<Value> {
    if output.contains("not found") || output.contains("invalid") {
        return None;
    }
    let name =
        capture(r"(?i)(?:name|service):\s*(.+)", output).unwrap_or_else(|| format!("Service_{id}"));
    let protocols = list_capture(r"(?i)protocols?:\s*(.+)", output, ',', true);
    let ports = list_capture(r"(?i)ports?:\s*(.+)", output, ',', false)
        .into_iter()
        .filter(|p| p != "0")
        .collect::<Vec<_>>();
    let categories = list_capture(r"(?i)categor(?:y|ies):\s*(.+)", output, ',', false);
    Some(
        json!({"id":id.to_string(),"name":name,"protocols":protocols,"ports":ports,"categories":categories,"content":output,"date":Utc::now().to_rfc3339(),"version":"collected"}),
    )
}

pub fn parse_isdb_range(output: &str) -> Option<i64> {
    capture(r"(?i)total.*?(\d+)", output)?
        .parse::<i64>()
        .ok()
        .filter(|n| *n > 0 && *n <= 1_000_000_000)
}
pub fn parse_isdb_version(output: &str) -> Option<String> {
    capture(r"(?i)(?:version|database):\s*([\d.]+)", output)
}
pub fn parse_fortios_version(output: &str) -> Option<String> {
    capture(r"Version:.*?v?([\d.]+)", output)
}
pub fn parse_panos_firmware(xml: &str) -> Option<String> {
    #[derive(Deserialize)]
    struct InfoResponse {
        result: InfoResult,
    }
    #[derive(Deserialize)]
    struct InfoResult {
        system: InfoSystem,
    }
    #[derive(Deserialize)]
    struct InfoSystem {
        #[serde(rename = "sw-version")]
        version: String,
    }
    if xml.len() > super::common::MAX_BODY_BYTES {
        return None;
    }
    quick_xml::de::from_str::<InfoResponse>(xml)
        .ok()
        .map(|r| r.result.system.version)
}

fn capture(pattern: &str, text: &str) -> Option<String> {
    regex::Regex::new(pattern)
        .ok()?
        .captures(text)?
        .get(1)
        .map(|m| m.as_str().trim().to_string())
}
fn list_capture(pattern: &str, text: &str, delimiter: char, uppercase: bool) -> Vec<String> {
    let Some(value) = capture(pattern, text) else {
        return Vec::new();
    };
    value
        .split(delimiter)
        .map(str::trim)
        .filter(|v| !v.is_empty() && *v != "-")
        .map(|v| {
            if uppercase {
                v.to_ascii_uppercase()
            } else {
                v.to_string()
            }
        })
        .collect()
}

pub fn dispatch(op: &str, input: &Value) -> Result<Value, String> {
    match op {
        "appid.panos.parse" => panos_cache(
            &text(&input["xml"]),
            &text(&input["device_id"]),
            &text(&input["firmware_version"]),
        ),
        "appid.panos.default_service" => {
            let (protocol, ports) =
                default_service(&text(&input["ident_by_ip_protocol"]), &text(&input["port"]));
            Ok(json!({"protocol":protocol,"ports":ports}))
        }
        "appid.panos.parse_firmware" => Ok(json!(parse_panos_firmware(&text(&input["xml"])))),
        "appid.fortigate.parse_entry" => {
            let entry =
                parse_isdb_entry(input["id"].as_i64().unwrap_or(0), &text(&input["output"]));
            Ok(json!({"found":entry.is_some(),"entry":entry}))
        }
        "appid.fortigate.parse_range" => Ok(json!(parse_isdb_range(&text(&input["output"])))),
        "appid.fortigate.parse_version" => Ok(json!(parse_isdb_version(&text(&input["output"])))),
        "appid.fortigate.parse_firmware" => {
            Ok(json!(parse_fortios_version(&text(&input["output"]))))
        }
        _ => Err("unsupported_operation".into()),
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
    #[test]
    fn parses_panos_applications_and_default_service() {
        let xml = r#"<response status="success"><result><application><version>8901-1234</version><entry><name>Web-Browsing</name><category>general-internet</category><subcategory>internet-utility</subcategory><technology>browser-based</technology><description>web</description><risk>2</risk><default><ident-by-ip-protocol>tcp</ident-by-ip-protocol><port>tcp/80,443</port></default><use-applications><member>ssl</member></use-applications><deprecated>no</deprecated></entry></application></result></response>"#;
        let result = panos_cache(xml, "panos-1", "11.0").unwrap();
        assert_eq!(
            result["cache"]["applications"]["web-browsing"]["default_ports"],
            json!(["80", "443"])
        );
        assert_eq!(
            result["cache"]["applications"]["web-browsing"]["parent_app"],
            "ssl"
        );
        assert_eq!(
            default_service("weird", "udp/0"),
            ("tcp".into(), Vec::<String>::new())
        );
    }
    #[test]
    fn parses_fortigate_isdb_fields_and_invalid_ids() {
        let entry = parse_isdb_entry(
            12,
            "Name: Google\nProtocols: TCP, UDP\nPorts: 80,443\nCategories: Search, Cloud",
        )
        .unwrap();
        assert_eq!(entry["protocols"], json!(["TCP", "UDP"]));
        assert!(parse_isdb_entry(13, "Service not found").is_none());
        assert_eq!(parse_isdb_range("Total entries: 9876"), Some(9876));
        assert_eq!(parse_isdb_range("Total entries: 12000"), Some(12000));
        assert_eq!(
            parse_isdb_version("Database: 12.00365").as_deref(),
            Some("12.00365")
        );
    }
}
