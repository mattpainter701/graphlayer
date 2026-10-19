//! Native ASA configuration normalization and evidence projection.
mod parser;
mod projection;
mod services;

pub use parser::{expand_address, parse_running_config, ParsedConfig};
pub use projection::parse_asa_with_hash;

pub fn service_port(protocol:&str,name:&str)->Option<String>{services::service_port_for(protocol,name)}
pub fn port_alternatives(fields:&[&str],protocol:&str,object:bool)->Option<(Vec<String>,usize)>{
    if object&&!fields.first().is_some_and(|f|matches!(f.to_ascii_lowercase().as_str(),"eq"|"range")){return None;}
    services::port_alternatives(fields,protocol)
}

use serde::{Deserialize, Serialize};
use std::collections::BTreeMap;

#[derive(Clone, Default, Serialize, Deserialize)]
#[serde(rename_all = "PascalCase")]
pub struct AddressObject {
    pub name: String,
    pub members: Vec<String>,
    pub member_lines: Vec<i32>,
    pub lines: Vec<i32>,
    pub unsupported_lines: Vec<i32>,
    pub group: bool,
}
#[derive(Clone, Default, Serialize, Deserialize)]
#[serde(rename_all = "PascalCase")]
pub struct ServiceObject {
    pub name: String,
    pub protocol: String,
    pub source_port: String,
    pub destination_port: String,
    pub member_line: i32,
    pub lines: Vec<i32>,
    pub unsupported_lines: Vec<i32>,
}
#[derive(Clone, Default, Serialize, Deserialize)]
#[serde(rename_all = "PascalCase")]
pub struct ServiceMember {
    pub kind: String,
    pub protocol: String,
    pub source_port: String,
    pub destination_port: String,
    pub reference: String,
    pub reference_kind: String,
    pub line: i32,
}
#[derive(Clone, Default, Serialize, Deserialize)]
#[serde(rename_all = "PascalCase")]
pub struct ServiceGroup {
    pub name: String,
    pub protocol: String,
    pub members: Vec<ServiceMember>,
    pub lines: Vec<i32>,
    pub unsupported_lines: Vec<i32>,
}
#[derive(Clone, Default, Serialize, Deserialize)]
#[serde(rename_all = "PascalCase")]
pub struct Ace {
    #[serde(rename = "ACLName")]
    pub acl_name: String,
    pub line: i32,
    pub order: i32,
    pub explicit_order: bool,
    pub raw: String,
    pub action: String,
    pub protocol: String,
    pub source: String,
    pub destination: String,
    pub source_port: String,
    pub destination_port: String,
    pub disabled: bool,
    pub enabled: bool,
    pub unsupported: Vec<String>,
    pub source_lines: Vec<i32>,
    pub service_group: String,
    pub service_object: String,
}
#[derive(Clone, Default, Serialize, Deserialize)]
#[serde(rename_all = "PascalCase")]
pub struct Binding {
    #[serde(rename = "ACLName")]
    pub acl_name: String,
    pub interface: String,
    pub direction: String,
    pub line: i32,
    pub supported: bool,
}
#[derive(Clone, Default, Serialize, Deserialize)]
#[serde(rename_all = "PascalCase")]
pub struct ConfigWarning {
    pub code: String,
    pub message: String,
    pub line: i32,
    #[serde(rename = "ACLName")]
    pub acl_name: String,
}
#[derive(Clone, Default, Serialize, Deserialize)]
#[serde(rename_all = "PascalCase")]
pub struct RawLine {
    pub number: i32,
    pub text: String,
}

fn unique<T: Clone + PartialEq>(items: &[T]) -> Vec<T> {
    let mut result = Vec::new();
    for item in items {
        if !result.contains(item) { result.push(item.clone()); }
    }
    result
}
fn warning(code: &str, message: &str, line: i32, acl: &str) -> ConfigWarning {
    ConfigWarning { code: code.into(), message: message.into(), line, acl_name: acl.into() }
}
fn harmless(fields: &[&str]) -> bool {
    fields.first().is_none_or(|f| matches!(f.to_ascii_lowercase().as_str(), "description"|"comment"|"tag"|"color"))
}
fn network_reference(expr: &str) -> (&str, &str) {
    match expr.split_once(':') {
        Some((kind,name)) if matches!(kind,"network_object"|"network_group") => (name,kind),
        _ => (expr,""),
    }
}
fn object_id(objects: &BTreeMap<String, AddressObject>, expr: &str) -> String {
    let raw = expr.trim_start_matches('@');
    let (name, kind) = network_reference(raw);
    let kind = if kind.is_empty() {
        if objects.get(name).is_some_and(|o| o.group) { "network_group" } else { "network_object" }
    } else {kind};
    format!("{kind}:{name}")
}
