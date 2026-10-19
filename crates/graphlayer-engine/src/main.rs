#![forbid(unsafe_code)]

use graphlayer_engine::{parse_asa_path, Input};
use serde::{Deserialize, Serialize};
use serde_json::Value;
use std::io::{self, BufRead, Read, Write};
use std::panic::{self, AssertUnwindSafe};

const PROTOCOL: &str = "graphlayer-engine-v1";
const MAX_INPUT: usize = 160 * 1024 * 1024;
const MAX_OUTPUT: usize = 8 * 1024 * 1024;

#[derive(Deserialize)]
#[serde(deny_unknown_fields)]
struct Request { protocol_version:String, operation:String, input:Value }
#[derive(Serialize)]
struct Response<'a>{
    protocol_version:&'a str,
    operation:&'a str,
    #[serde(skip_serializing_if="Option::is_none")]
    result:Option<Value>,
    #[serde(skip_serializing_if="Option::is_none")]
    error:Option<String>,
    #[serde(skip_serializing_if="Option::is_none")]
    error_kind:Option<&'a str>,
}

fn main(){
    // Protocol failures never print source text, network payloads or credentials.
    panic::set_hook(Box::new(|_|{}));
    let arguments:Vec<String>=std::env::args().skip(1).collect();
    if arguments.is_empty(){
        if panic::catch_unwind(run_once).is_err(){emit(error("parse_asa_path","internal_error"));}
    }else if arguments==["--session"]{
        if panic::catch_unwind(run_session).is_err(){emit(error("session","internal_error"));}
    }else{emit(error("parse_asa_path","invalid_request"));}
}
fn run_once(){
    let mut bytes=Vec::new();
    if io::stdin().take((MAX_INPUT+1)as u64).read_to_end(&mut bytes).is_err(){emit(error("parse_asa_path","invalid_request"));return;}
    emit(handle(&bytes,None));
}
fn run_session(){
    let mut dispatcher=graphlayer_engine::connectors::SessionDispatcher::new();
    let mut reader=io::stdin().lock();
    loop{
        let mut frame=Vec::new();
        match reader.by_ref().take((MAX_INPUT+1)as u64).read_until(b'\n',&mut frame){
            Ok(0)=>break,
            Ok(_)=>{
                let oversized=frame.len()>MAX_INPUT;
                let response=panic::catch_unwind(AssertUnwindSafe(||handle(&frame,Some(&mut dispatcher)))).unwrap_or_else(|_|error("session","internal_error"));
                emit(response);
                if oversized{break;}
            },
            Err(_)=>{emit(error("session","invalid_request"));break;},
        }
    }
}
fn handle(bytes:&[u8],session:Option<&mut graphlayer_engine::connectors::SessionDispatcher>)->Response<'static>{
    if bytes.len()>MAX_INPUT{return error("parse_asa_path","input_too_large");}
    let request:Request=match serde_json::from_slice(bytes){Ok(value)=>value,Err(_)=>return error("parse_asa_path","invalid_request")};
    // A fixed allowlist prevents request-derived strings in protocol diagnostics.
    let operation=match known_operation(&request.operation){Some(operation)=>operation,None=>return error("parse_asa_path","unsupported_request")};
    if request.protocol_version!=PROTOCOL{return error(operation,"unsupported_request");}
    let result=if operation=="parse_asa_path"{
        if bytes.len()>12*1024*1024{return error(operation,"input_too_large");}
        let input:Input=match serde_json::from_value(request.input){Ok(input)=>input,Err(_)=>return error(operation,"invalid_request")};
        if !valid_path_input(&input){return error(operation,"invalid_request");}
        serde_json::to_value(parse_asa_path(input)).map_err(|_|"internal_error".into())
    }else if let Some(session)=session{
        if operation.starts_with("ssh.")||operation.starts_with("api."){session.dispatch(operation,request.input)}
        else{graphlayer_engine::dispatch::dispatch(operation,request.input)}
    }else{graphlayer_engine::dispatch::dispatch(operation,request.input)};
    match result{
        Ok(result)=>Response{protocol_version:PROTOCOL,operation,result:Some(result),error:None,error_kind:None},
        Err(message)=>{
            let protocol=matches!(message.as_str(),"invalid_request"|"internal_error"|"unsupported_operation"|"input_too_large"|"output_too_large");
            if message.len()>4096{return error(operation,"internal_error");}
            Response{protocol_version:PROTOCOL,operation,result:None,error:Some(message),error_kind:if protocol{None}else{Some("domain")}}
        }
    }
}
fn known_operation(operation:&str)->Option<&'static str>{
    const OPERATIONS:&[&str]=&[
        "parse_asa_path","parse_config","parse_asa","parse_asa_raw","parse_panos","parse_fortigate","asa.expand_address","asa.sanitize_line","asa.evidence_id","asa.service_port","asa.port_alternatives","validate_inventory","validate_path_config",
        "validate_scope","validate_policy","evaluate_policy","evaluate_path","evaluate_return","explore_path","explore_return","evaluate_network","explore_network",
        "netbox.normalize","netbox.collect","netbox.ping","netbox.fetch_endpoint_page","netbox.fetch_endpoint",
        "servicenow.normalize","servicenow.collect","servicenow.ping","servicenow.fetch_table_page","servicenow.fetch_table",
        "servicenow.load_state","servicenow.save_state","servicenow.resolve_updated_since","api.config","solarwinds.trigger",
        "connector.make_record","connector.record_fingerprint","connector.correlation_keys","connector.dedupe_records","connector.collection_report","connector.filter_devices","connector.stale","connector.delta",
        "connector.fingerprint","connector.build_record","connector.deduplicate","connector.correlation_report","connector.filter_confidence","connector.staleness_decay","connector.build_payload","connector.delta_sync","connector.dedup_key","connector.normalize_hostname","connector.normalize_correlation_value","connector.validate_required_fields","connector.normalize_path","connector.normalize_type",
        "fileshare.normalize_path","fileshare.matches_patterns","fileshare.create_local_filename","fileshare.connect","fileshare.collect","fileshare.ping",
        "solarwinds.sanitize_filename","solarwinds.ping","solarwinds.collect",
        "fileshare.disconnect","solarwinds.query","solarwinds.devices","solarwinds.config","solarwinds.save_config",
        "ssh.connect","ssh.execute","ssh.disconnect","ssh.device_info","ssh.config_command",
        "ssh.close_all","ssh.clean_output","ssh.device_info_command",
        "ssh.prompt_pattern","ssh.parse_output","ssh.supported_commands","ssh.capabilities","ssh.open_shell","ssh.shell_execute","ssh.get_config","path.is_asa_vendor",
        "api.connect","api.authenticate","api.request","api.health","api.collect","api.vendor_operation","api.disconnect",
        "api.device_info","api.supported_commands","api.inventory",
        "appid.panos.parse","appid.panos.default_service","appid.fortigate.parse_entry","appid.fortigate.parse_range","appid.fortigate.parse_version","appid.fortigate.parse_firmware","api.collect_appid","ssh.collect_isdb",
        "appid.panos.parse_firmware",
        "ssh.close_shell",
    ];
    OPERATIONS.iter().copied().find(|candidate|*candidate==operation)
}
fn valid_path_input(input:&Input)->bool{
    if input.artifact_sha256.len()!=64||!input.artifact_sha256.bytes().all(|b|b.is_ascii_digit()||(b'a'..=b'f').contains(&b)){return false;}
    if input.lines.len()>50_000{return false;}
    let mut size=0usize;
    for(index,line)in input.lines.iter().enumerate(){
        if line.number!=index as i32+1||line.text.len()>8192{return false;}
        size=match size.checked_add(line.text.len()+usize::from(index+1<input.lines.len())){Some(total)if total<=2*1024*1024=>total,_=>return false};
    }true
}
fn error(operation:&'static str,message:&str)->Response<'static>{Response{protocol_version:PROTOCOL,operation,result:None,error:Some(message.into()),error_kind:None}}
fn emit(response:Response<'static>){
    let operation=response.operation;
    let bytes=match serde_json::to_vec(&response){Ok(bytes)if bytes.len()<MAX_OUTPUT=>bytes,_=>serde_json::to_vec(&error(operation,"output_too_large")).unwrap_or_default()};
    let mut stdout=io::stdout().lock();let _=stdout.write_all(&bytes);let _=stdout.write_all(b"\n");let _=stdout.flush();
}
