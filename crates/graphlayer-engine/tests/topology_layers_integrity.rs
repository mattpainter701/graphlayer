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
fn host_mac_addresses_are_unicast_and_compared_as_bytes() {
    let original = dataset();
    let mut multicast = original.clone();
    multicast["hosts"][0]["mac"] = json!("03:20:01:00:01:01");
    let request = json!({"dataset":multicast,"view":"l2"});
    assert_eq!(project_layers(request).unwrap_err(), "invalid_mac");
    let mut duplicate = original.clone();
    duplicate["hosts"][1]["mac"] = original["hosts"][9]["mac"]
        .as_str()
        .unwrap()
        .to_uppercase()
        .into();
    let request = json!({"dataset":duplicate,"view":"l2"});
    assert_eq!(project_layers(request).unwrap_err(), "invalid_mac");
}

#[test]
fn zones_and_subnet_membership_cannot_be_replaced_by_unrelated_labels() {
    let original = dataset();
    for zones in [
        json!([null]),
        json!(["unrelated"]),
        json!(["engineering", "engineering"]),
    ] {
        let mut bad = original.clone();
        bad["zones"] = zones;
        assert!(project_layers(json!({"dataset":bad,"view":"zone"})).is_err());
    }
    let mut outside = original.clone();
    outside["hosts"][0]["ip"] = json!("10.99.0.1");
    assert!(project_layers(json!({"dataset":outside,"view":"l2"})).is_err());
}

#[test]
fn filters_and_selected_path_keep_inventory_and_evidence_ownership() {
    let original = dataset();
    let late =
        project_layers(json!({"dataset":original,"view":"l2","search":"host-1000","limit":25}))
            .unwrap();
    assert_eq!(late["hosts"][0]["id"], "host-1000");
    assert!(late["nodes"]
        .as_array()
        .unwrap()
        .iter()
        .any(|n| n["id"] == "host-1000"));
    let zones =
        project_layers(json!({"dataset":original,"view":"l3","zone":"engineering"})).unwrap();
    assert!(zones["nodes"]
        .as_array()
        .unwrap()
        .iter()
        .all(|n| n["zone"] == "engineering"));
    let path = project_layers(json!({"dataset":original,"view":"l2","limit":25,"source_host_id":"host-0001","destination_host_id":"host-0075"})).unwrap();
    assert_eq!(path["path"]["links"].as_array().unwrap().len(), 4);
    let hops = path["path"]["hops"].as_array().unwrap();
    assert_eq!(hops.last().unwrap()["node_id"], "host-0075");
    let destination_ref = &original["hosts"][74]["evidence"][0];
    assert!(hops.last().unwrap()["evidence"]
        .as_array()
        .unwrap()
        .contains(destination_ref));
    let cross = project_layers(json!({"dataset":original,"view":"l2","source_host_id":"host-0001","destination_host_id":"host-1000"})).unwrap();
    assert_eq!(cross["path"]["status"], "requires_l3");
    assert!(cross["path"]["links"].as_array().unwrap().is_empty());
}
