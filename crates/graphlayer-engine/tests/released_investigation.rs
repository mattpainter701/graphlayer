use flate2::read::GzDecoder;
use serde_json::{json, Value};
use std::io::Read;

fn evaluate(operation: &str, input: Value) -> Result<Value, String> {
    graphlayer_engine::dispatch::dispatch(operation, input)
}

// The released Go PAN-OS parser appended top-level evidence while ranging a
// map. Captured cases 867/869 and 873/874 have identical inputs and identical
// evidence objects in different orders. Interfaces and connected routes are
// appended from the same map. Compare those collections as multisets;
// keep every evidence field, duplicate, reference and all other arrays strict.
fn normalize_unordered_evidence(operation: &str, value: &mut Value) {
    if operation == "parse_panos" {
        if let Some(evidence) = value.pointer_mut("/result/evidence").and_then(Value::as_array_mut) {
            evidence.sort_by_key(Value::to_string);
        }
        if let Some(interfaces) = value.pointer_mut("/result/path/interfaces").and_then(Value::as_array_mut) {
            interfaces.sort_by_key(Value::to_string);
        }
        if let Some(routes) = value.pointer_mut("/result/path/routes").and_then(Value::as_array_mut) {
            let positions: Vec<usize> = routes.iter().enumerate().filter(|(_, route)| route["kind"] == "connected").map(|(index, _)| index).collect();
            let mut connected: Vec<Value> = positions.iter().map(|index| routes[*index].clone()).collect();
            connected.sort_by_key(Value::to_string);
            for (index, route) in positions.into_iter().zip(connected) { routes[index] = route; }
        }
    }
}

#[test]
fn unordered_parser_inventory_keeps_values_duplicates_and_static_order_strict() {
    let original = json!({"result":{"evidence":[{"id":"b"},{"id":"a"},{"id":"a"}],"path":{"interfaces":[{"name":"b"},{"name":"a"}],"routes":[{"kind":"connected","id":"b"},{"kind":"connected","id":"a"},{"kind":"static","id":"z"},{"kind":"static","id":"y"}]},"chains":[{"id":"z"},{"id":"a"}]}});
    let mut normalized = original.clone();
    normalize_unordered_evidence("parse_panos", &mut normalized);
    assert_eq!(normalized["result"]["evidence"].as_array().unwrap().len(), 3);
    assert_eq!(normalized["result"]["path"]["routes"][2]["id"], "z");
    assert_eq!(normalized["result"]["chains"], original["result"]["chains"]);
    let mut changed = normalized.clone();
    changed["result"]["evidence"][0]["id"] = json!("different");
    assert!(first_difference(&normalized, &changed, "").is_some());
    let mut other_vendor = original.clone();
    normalize_unordered_evidence("parse_fortigate", &mut other_vendor);
    assert_eq!(other_vendor, original);
}

fn first_difference(expected: &Value, actual: &Value, pointer: &str) -> Option<String> {
    match (expected, actual) {
        (Value::Object(a), Value::Object(b)) => {
            for key in a.keys().chain(b.keys()) {
                let path = format!("{pointer}/{key}");
                match (a.get(key), b.get(key)) {
                    (Some(x), Some(y)) => {
                        if let Some(difference) = first_difference(x, y, &path) {
                            return Some(difference);
                        }
                    }
                    _ => return Some(format!("{path}: field presence differs")),
                }
            }
            None
        }
        (Value::Array(a), Value::Array(b)) => {
            if a.len() != b.len() {
                return Some(format!(
                    "{pointer}: array length {} != {}",
                    a.len(),
                    b.len()
                ));
            }
            a.iter()
                .zip(b)
                .enumerate()
                .find_map(|(i, (x, y))| first_difference(x, y, &format!("{pointer}/{i}")))
        }
        _ if expected == actual => None,
        _ => Some(format!(
            "{pointer}: expected {}, actual {}",
            expected.to_string().chars().take(300).collect::<String>(),
            actual.to_string().chars().take(300).collect::<String>()
        )),
    }
}

#[test]
fn all_released_parser_and_investigation_contracts_are_preserved() {
    let mut decoded = String::new();
    GzDecoder::new(&include_bytes!("../testdata/released-investigation.ndjson.gz")[..])
        .read_to_string(&mut decoded)
        .unwrap();
    let mut failures = Vec::new();
    let mut counts = std::collections::BTreeMap::<String, (usize, usize)>::new();
    for (index, line) in decoded.lines().enumerate() {
        let case: Value = serde_json::from_str(line).unwrap();
        let operation = case["operation"].as_str().unwrap();
        let result = std::panic::catch_unwind(std::panic::AssertUnwindSafe(|| {
            evaluate(operation, case["input"].clone())
        }))
        .unwrap_or_else(|_| Err("__panic__".into()));
        let mut actual = match result {
            Ok(value) => json!({"result":value}),
            Err(error) => json!({"error":error}),
        };
        let mut expected = if case.get("error").is_some() {
            json!({"error":case["error"]})
        } else {
            json!({"result":case["result"]})
        };
        normalize_unordered_evidence(operation, &mut actual);
        normalize_unordered_evidence(operation, &mut expected);
        let count = counts.entry(operation.into()).or_default();
        count.0 += 1;
        if let Some(difference) = first_difference(&expected, &actual, "") {
            failures.push(json!({"index":index,"operation":operation,"difference":difference,"actual":actual}));
        } else {
            count.1 += 1;
        }
    }
    std::fs::write(
        "released-parity-failures.json",
        serde_json::to_vec_pretty(&failures).unwrap(),
    )
    .unwrap();
    eprintln!("Released contract parity: {counts:?}");
    assert!(
        failures.is_empty(),
        "{} released contracts differ; first failures: {}",
        failures.len(),
        serde_json::to_string(&failures.iter().take(25).collect::<Vec<_>>()).unwrap()
    );
}
