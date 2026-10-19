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
        "parse_config"=>{
            let bytes=base64::engine::general_purpose::STANDARD.decode(input["config_base64"].as_str().ok_or("invalid_request")?).map_err(|_|"invalid_request")?;
            if bytes.len()>2<<20{return Err("configuration exceeds input budget".into());}
            let text=source_text(&bytes).replace("\r\n","\n");
            let mut vendor=input["vendor"].as_str().unwrap_or_default().trim().to_lowercase();
            if vendor.is_empty()||vendor=="auto"{
                let(mut pan,mut forti,mut asa)=(false,false,false);
                for line in text.split('\n').map(str::trim){
                    pan|=line.starts_with("set network ")||line.starts_with("set rulebase ")||line.starts_with("set deviceconfig ");
                    forti|=line.starts_with("config system interface")||line.starts_with("config firewall ");
                    asa|=line.starts_with("access-list ")||line.starts_with("access-group ");
                }
                if usize::from(pan)+usize::from(forti)+usize::from(asa)!=1{return Err("configuration format is absent or ambiguous".into());}
                vendor=if pan{"paloalto-panos"}else if forti{"fortinet-fortigate"}else{"cisco-asa"}.into();
            }
            let operation=match vendor.as_str(){"cisco-asa"=>"parse_asa","paloalto-panos"=>"parse_panos","fortinet-fortigate"=>"parse_fortigate",_=>return Err("unsupported configuration format".into())};
            dispatch(operation,input)
        },
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
            let mut config=input["config"].clone();normalize_asa_collections(&mut config);
            let parsed=serde_json::from_value(config).map_err(|_|"invalid_request")?;
            let expr=input["expression"].as_str().ok_or("invalid_request")?;
            let(addresses,lines,error)=crate::asa_config::expand_address(&parsed,expr);
            Ok(serde_json::json!({"addresses":addresses,"lines":lines,"error":error}))
        },
        "asa.sanitize_line"=>Ok(serde_json::json!({"text":crate::sanitize_line(input["text"].as_str().ok_or("invalid_request")?)})),
        "asa.evidence_id"=>{
            let hash=input["hash"].as_str().ok_or("invalid_request")?;
            let lines:Vec<i32>=serde_json::from_value(input["lines"].clone()).map_err(|_|"invalid_request")?;
            Ok(serde_json::json!({"id":crate::evidence_id(hash,&lines)}))
        },
        "asa.service_port"=>{
            let port=crate::asa_config::service_port(input["protocol"].as_str().unwrap_or_default(),input["name"].as_str().unwrap_or_default());
            Ok(serde_json::json!({"ok":port.is_some(),"port":port.unwrap_or_default()}))
        },
        "asa.port_alternatives"=>{
            let fields:Vec<String>=serde_json::from_value(input["fields"].clone()).map_err(|_|"invalid_request")?;
            let fields:Vec<&str>=fields.iter().map(String::as_str).collect();
            let result=crate::asa_config::port_alternatives(&fields,input["protocol"].as_str().unwrap_or_default(),input["object"].as_bool().unwrap_or(false));
            Ok(match result{Some((ports,consumed))=>serde_json::json!({"ports":ports,"consumed":consumed,"ok":true}),None=>serde_json::json!({"ports":null,"consumed":0,"ok":false})})
        },
        "validate_inventory"=>crate::model_validation::validate_inventory(input),
        "validate_path_config"=>crate::model_validation::validate_path_config(input),
        "path.is_asa_vendor"=>Ok(serde_json::json!(matches!(input["vendor"].as_str().unwrap_or_default().trim().to_ascii_lowercase().as_str(),"cisco-asa"|"cisco_asa"|"asa"))),
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

fn normalize_asa_collections(value:&mut Value){
    match value{
        Value::Object(fields)=>for(key,value)in fields{
            if value.is_null(){
                if matches!(key.as_str(),"ACLs"|"Objects"|"ServiceObjects"|"ServiceGroups"){*value=serde_json::json!({});}
                else if matches!(key.as_str(),"Lines"|"Bindings"|"Warnings"|"Members"|"MemberLines"|"UnsupportedLines"|"SourceLines"|"Unsupported"){*value=serde_json::json!([]);}
            }normalize_asa_collections(value);
        },
        Value::Array(values)=>for value in values{normalize_asa_collections(value)},
        _=>{},
    }
}
