use graphlayer_engine::investigation_engine::evaluate_policy;
use serde_json::json;

#[test]
fn investigation_policy_preserves_first_match_and_predicate_trace() {
    let scope = json!({"snapshot_id":"snap","device_id":"fw","context":"default"});
    let input = json!({
        "bundle":{"scope":scope,"complete":true,"default_action":"deny","rules":[
            {"id":"first","position":10,"enabled":true,"action":"deny","source":"any4","destination":"10.0.0.0/8","protocol":"tcp","destination_port":"443","evidence_refs":["ev:first"]},
            {"id":"later","position":20,"enabled":true,"action":"permit","source":"any","destination":"any","protocol":"tcp","source_port":"","destination_port":"443"}
        ]},
        "flow":{"scope":scope,"source":"192.0.2.2","destination":"10.1.2.3","protocol":"tcp","source_port":50000,"destination_port":443}
    });
    let result = evaluate_policy(input).expect("valid policy input");
    assert_eq!(result["disposition"], "deny");
    assert_eq!(result["decisive_rule"], "first");
    assert_eq!(result["considered_rules"], json!(["first"]));
    assert_eq!(result["rule_traces"][0]["outcome"], "matched");
    assert_eq!(result["evidence_refs"], json!(["ev:first"]));
}

#[test]
fn investigation_policy_known_mismatch_removes_unresolved_candidate() {
    let scope = json!({"snapshot_id":"snap","device_id":"fw","context":"default"});
    let input = json!({
        "bundle":{"scope":scope,"complete":true,"default_action":"deny","rules":[
            {"id":"object","position":1,"enabled":true,"action":"deny","source":"192.0.2.0/24","destination":"unresolved-name","protocol":"tcp"},
            {"id":"permit","position":2,"enabled":true,"action":"permit","source":"any","destination":"any","protocol":"tcp"}
        ]},
        "flow":{"scope":scope,"source":"198.51.100.2","destination":"10.0.0.2","protocol":"tcp","source_port":50000,"destination_port":443}
    });
    let result = evaluate_policy(input).expect("valid policy input");
    assert_eq!(result["disposition"], "permit");
    assert_eq!(result["decisive_rule"], "permit");
    assert_eq!(result["rule_traces"][0]["predicates"][0]["state"], "not_match");
    assert_eq!(result["rule_traces"][0]["predicates"][1]["state"], "unresolved");
}
