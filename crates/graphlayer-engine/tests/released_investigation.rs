use base64::Engine;
use flate2::read::GzDecoder;
use graphlayer_engine::{investigation_engine as engine, vendor_parsers as parser};
use serde_json::{json, Value};
use std::io::Read;

fn evaluate(operation: &str, input: Value) -> Result<Value, String> {
    match operation {
        "validate_scope" => engine::validate_scope(input),
        "validate_policy" => engine::validate_policy(input),
        "evaluate_policy" => engine::evaluate_policy(input),
        "evaluate_path" => engine::evaluate_path(input),
        "evaluate_return" => engine::evaluate_return(input),
        "explore_path" => engine::explore_path(input),
        "explore_return" => engine::explore_return(input),
        "evaluate_network" => engine::evaluate_network(input),
        "explore_network" => engine::explore_network(input),
        "parse_asa" | "parse_panos" | "parse_fortigate" => {
            use sha2::{Digest, Sha256};
            let bytes = base64::engine::general_purpose::STANDARD.decode(input["config_base64"].as_str().unwrap()).unwrap();
            let hash = format!("{:x}", Sha256::digest(&bytes));
            let text = String::from_utf8_lossy(&bytes);
            if operation == "parse_asa" { graphlayer_engine::asa_config::parse_asa_with_hash(&text, &hash) }
            else if operation == "parse_panos" { parser::parse_panos_with_hash(&text, &hash) }
            else { parser::parse_fortigate_with_hash(&text, &hash) }
        }
        _ => Err("unsupported_operation".into()),
    }
}

fn first_difference(expected: &Value, actual: &Value, pointer: &str) -> Option<String> {
    match (expected, actual) {
        (Value::Object(a),Value::Object(b)) => {
            for key in a.keys().chain(b.keys()) {
                let path = format!("{pointer}/{key}");
                match (a.get(key),b.get(key)) {
                    (Some(x),Some(y)) => if let Some(difference)=first_difference(x,y,&path) { return Some(difference); },
                    _ => return Some(format!("{path}: field presence differs")),
                }
            }
            None
        },
        (Value::Array(a),Value::Array(b)) => {
            if a.len()!=b.len() {return Some(format!("{pointer}: array length {} != {}",a.len(),b.len()));}
            a.iter().zip(b).enumerate().find_map(|(i,(x,y))|first_difference(x,y,&format!("{pointer}/{i}")))
        },
        _ if expected==actual => None,
        _ => Some(format!("{pointer}: expected {}, actual {}",expected.to_string().chars().take(300).collect::<String>(),actual.to_string().chars().take(300).collect::<String>())),
    }
}

#[test]
fn all_released_parser_and_investigation_contracts_are_preserved() {
    let mut decoded=String::new();
    GzDecoder::new(&include_bytes!("../testdata/released-investigation.ndjson.gz")[..]).read_to_string(&mut decoded).unwrap();
    let mut failures=Vec::new();
    let mut counts=std::collections::BTreeMap::<String,(usize,usize)>::new();
    for (index,line) in decoded.lines().enumerate() {
        let case:Value=serde_json::from_str(line).unwrap();
        let operation=case["operation"].as_str().unwrap();
        let result=evaluate(operation,case["input"].clone());
        let actual=match result {Ok(value)=>json!({"result":value}),Err(error)=>json!({"error":error})};
        let expected=if case.get("error").is_some(){json!({"error":case["error"]})}else{json!({"result":case["result"]})};
        let count=counts.entry(operation.into()).or_default();count.0+=1;
        if let Some(difference)=first_difference(&expected,&actual,"") {
            failures.push(json!({"index":index,"operation":operation,"difference":difference}));
        } else {count.1+=1;}
    }
    std::fs::write("released-parity-failures.json",serde_json::to_vec_pretty(&failures).unwrap()).unwrap();
    eprintln!("Released contract parity: {counts:?}");
    assert!(failures.is_empty(),"{} released contracts differ; first failures: {}",failures.len(),serde_json::to_string(&failures.iter().take(25).collect::<Vec<_>>()).unwrap());
}
