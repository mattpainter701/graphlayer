use base64::Engine;
use serde_json::Value;
use sha2::{Digest, Sha256};

/// Go's historical JSON encoder replaced each invalid UTF-8 byte separately.
/// Preserve that persisted source representation while hashing the raw upload.
pub fn source_text(bytes: &[u8]) -> String {
    let mut output=String::new();let mut rest=bytes;
    while !rest.is_empty(){
        match std::str::from_utf8(rest){
            Ok(valid)=>{output.push_str(valid);break;},
            Err(error)=>{
                let valid=error.valid_up_to();
                output.push_str(std::str::from_utf8(&rest[..valid]).unwrap_or_default());
                let length=error.error_len().unwrap_or(rest.len()-valid);
                for _ in 0..length{output.push('\u{fffd}');}
                rest=&rest[valid+length..];
            }
        }
    }output
}

pub fn dispatch(operation:&str,input:Value)->Result<Value,String>{
    use crate::investigation_engine as e;
    match operation{
        "parse_asa"|"parse_asa_raw"|"parse_panos"|"parse_fortigate"=>{
            let encoded=input.get("config_base64").and_then(Value::as_str).ok_or("invalid_request")?;
            if encoded.len()>((2<<20)+2)/3*4 {return Err(format!("{} config exceeds 2097152 bytes",if operation.starts_with("parse_asa"){"ASA"}else if operation=="parse_panos"{"PAN-OS"}else{"FortiGate"}));}
            let bytes=base64::engine::general_purpose::STANDARD.decode(encoded).map_err(|_|"invalid_request")?;
            let hash=format!("{:x}",Sha256::digest(&bytes));let text=source_text(&bytes);
            match operation{
                "parse_asa"=>crate::asa_config::parse_asa_with_hash(&text,&hash),
                "parse_asa_raw"=>serde_json::to_value(crate::asa_config::parse_running_config(&text,&hash)?).map_err(|_|"internal_error".into()),
                "parse_panos"=>crate::vendor_parsers::parse_panos_with_hash(&text,&hash),
                _=>crate::vendor_parsers::parse_fortigate_with_hash(&text,&hash),
            }
        },
        "asa.expand_address"=>{
            let parsed=serde_json::from_value(input["config"].clone()).map_err(|_|"invalid_request")?;
            let expr=input["expression"].as_str().ok_or("invalid_request")?;
            let(addresses,lines,error)=crate::asa_config::expand_address(&parsed,expr);
            Ok(serde_json::json!({"addresses":addresses,"lines":lines,"error":error}))
        },
        "asa.sanitize_line"=>Ok(serde_json::json!({"text":crate::sanitize_line(input["text"].as_str().ok_or("invalid_request")?)})),
        "validate_scope"=>e::validate_scope(input),
        "validate_policy"=>e::validate_policy(input),
        "evaluate_policy"=>e::evaluate_policy(input),
        "evaluate_path"=>e::evaluate_path(input),
        "evaluate_return"=>e::evaluate_return(input),
        "explore_path"=>e::explore_path(input),
        "explore_return"=>e::explore_return(input),
        "evaluate_network"=>e::evaluate_network(input),
        "explore_network"=>e::explore_network(input),
        operation if ["connector.","netbox.","servicenow.","solarwinds.","fileshare.","ssh.","api."].iter().any(|prefix|operation.starts_with(prefix))=>crate::connectors::dispatch(operation,input),
        _=>Err("unsupported_operation".into()),
    }
}
