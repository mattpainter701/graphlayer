use graphlayer_engine::topology_layers::{layer_sample, project_layers};
use serde_json::{json, Value};

fn dataset() -> Value {
    layer_sample(json!({})).unwrap()["dataset"].clone()
}

#[test]
fn incomplete_and_forged_auxiliary_facts_fail_closed() {
    let original = dataset();
    for (collection, field) in [
        ("hosts", "evidence"),
        ("devices", "evidence"),
        ("networks", "evidence"),
        ("links", "evidence"),
    ] {
        let mut bad = original.clone();
        bad[collection][0][field] = json!([]);
        assert!(project_layers(json!({"dataset":bad,"view":"l2"})).is_err());
    }
    for (field, replacement) in [
        ("network_id", json!("net-02")),
        ("zone", json!("operations")),
        ("subnet", json!("10.20.2.0/24")),
        ("attachment", json!("sw-01-b")),
    ] {
        let mut bad = original.clone();
        bad["hosts"][0][field] = replacement.clone();
        if field == "attachment" {
            bad["links"][0]["to"] = replacement;
        }
        assert!(project_layers(json!({"dataset":bad,"view":"l2"})).is_err());
    }
    for key in ["devices", "networks", "zones", "sources", "hosts", "links"] {
        for replacement in [Value::Null, json!("invalid"), json!({})] {
            let mut bad = original.clone();
            bad[key] = replacement;
            assert!(project_layers(json!({"dataset":bad,"view":"l3"})).is_err());
        }
    }
}

#[test]
fn filters_and_selected_path_keep_inventory_and_evidence_ownership() {
    let original = dataset();
    let late = project_layers(json!({"dataset":original,"view":"l2","search":"host-1000","limit":25})).unwrap();
    assert_eq!(late["hosts"][0]["id"], "host-1000");
    assert!(late["nodes"].as_array().unwrap().iter().any(|n| n["id"] == "host-1000"));
    let zones = project_layers(json!({"dataset":original,"view":"l3","zone":"engineering"})).unwrap();
    assert!(zones["nodes"].as_array().unwrap().iter().all(|n| n["zone"] == "engineering"));
    let path = project_layers(json!({"dataset":original,"view":"l2","limit":25,"source_host_id":"host-0001","destination_host_id":"host-0075"})).unwrap();
    assert_eq!(path["path"]["links"].as_array().unwrap().len(), 4);
    let hops = path["path"]["hops"].as_array().unwrap();
    assert_eq!(hops.last().unwrap()["node_id"], "host-0075");
    let destination_ref = &original["hosts"][74]["evidence"][0];
    assert!(hops.last().unwrap()["evidence"].as_array().unwrap().contains(destination_ref));
    let cross = project_layers(json!({"dataset":original,"view":"l2","source_host_id":"host-0001","destination_host_id":"host-1000"})).unwrap();
    assert_eq!(cross["path"]["status"], "requires_l3");
    assert!(cross["path"]["links"].as_array().unwrap().is_empty());
}
