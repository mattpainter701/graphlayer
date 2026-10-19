use serde_json::{json, Value};

#[test]
fn servicenow_fixture_normalization_preserves_reference_ids_and_evidence() {
    let devices: Value = serde_json::from_str(include_str!(
        "../../../tests/fixtures/servicenow/cmdb_ci_server_page1.json"
    ))
    .unwrap();
    let relationships: Value = serde_json::from_str(include_str!(
        "../../../tests/fixtures/servicenow/cmdb_rel_ci_page1.json"
    ))
    .unwrap();
    let input = json!({
        "connector_data":{"connector_type":"servicenow_cmdb","host":"http://127.0.0.1","payload":{"relationship_table":"cmdb_rel_ci"}},
        "raw_data":{"updated_since":"2024-01-01T00:00:00+00:00","devices":devices["result"],"relationships":relationships["result"]}
    });
    let out = crate::connectors::dispatch("servicenow.normalize", input).unwrap();
    assert_eq!(out["count"], 2);
    assert_eq!(out["devices"][0]["payload"]["device_type"], "server");
    assert_eq!(out["devices"][0]["payload"]["vendor"], "Dell");
    assert_eq!(out["relationships"][0]["payload"]["source_id"], "sw-001");
    assert_eq!(
        out["relationships"][0]["payload"]["relationship_type"],
        "Runs on::Runs"
    );
    assert_eq!(
        out["relationships"][0]["metadata"]["source_table"],
        "cmdb_rel_ci"
    );
}
