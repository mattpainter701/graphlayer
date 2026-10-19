use serde_json::{json, Value};

#[test]
fn netbox_fixture_normalization_preserves_inventory_links_and_ip_assignments() {
    let devices: Value = serde_json::from_str(include_str!(
        "../../../tests/fixtures/netbox/dcim_devices_page1.json"
    ))
    .unwrap();
    let interfaces: Value = serde_json::from_str(include_str!(
        "../../../tests/fixtures/netbox/dcim_interfaces_page1.json"
    ))
    .unwrap();
    let cables: Value = serde_json::from_str(include_str!(
        "../../../tests/fixtures/netbox/dcim_cables_page1.json"
    ))
    .unwrap();
    let prefixes: Value = serde_json::from_str(include_str!(
        "../../../tests/fixtures/netbox/ipam_prefixes_page1.json"
    ))
    .unwrap();
    let addresses: Value = serde_json::from_str(include_str!(
        "../../../tests/fixtures/netbox/ipam_ip_addresses_page1.json"
    ))
    .unwrap();
    let input = json!({
        "connector_data":{"connector_type":"netbox","host":"http://127.0.0.1","payload":{}},
        "raw_data":{"devices":devices["results"],"interfaces":interfaces["results"],"cables":cables["results"],"prefixes":prefixes["results"],"ip_addresses":addresses["results"]}
    });
    let out = crate::connectors::dispatch("netbox.normalize", input).unwrap();
    assert_eq!(out["count"], 5);
    assert_eq!(out["devices"][0]["payload"]["device_type"], "switch");
    assert_eq!(out["devices"][0]["payload"]["management_ip"], "10.0.0.2/24");
    assert_eq!(
        out["interfaces"][0]["payload"]["ip_addresses"][0]["address"],
        "10.0.0.2/24"
    );
    assert_eq!(
        out["links"][0]["payload"]["termination_b"]["device_name"],
        "edge-router-01"
    );
    assert_eq!(out["subnets"][0]["payload"]["site"], "nyc-1");
    assert!(out["records"][0]["raw_fingerprint"]
        .as_str()
        .unwrap()
        .starts_with("sha256:"));
}
