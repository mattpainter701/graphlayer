//! Rust investigation evaluators. Inputs and outputs intentionally use JSON
//! values at this boundary so older persisted contracts keep their shape.
mod policy;
mod path;
mod network;
mod vendor_path;

pub(crate) fn omit_empty_fields(value: &mut serde_json::Value) {
    let Some(object) = value.as_object_mut() else { return; };
    for key in ["evidence_refs", "candidate_ids", "nat_rule_ids"] {
        if object.get(key).and_then(serde_json::Value::as_array).is_some_and(|a| a.is_empty()) { object.remove(key); }
    }
    if let Some(stages) = object.get_mut("stages").and_then(serde_json::Value::as_array_mut) {
        for stage in stages {
            if let Some(fields) = stage.as_object_mut() {
                for key in ["evidence_refs", "candidate_ids"] {
                    if fields.get(key).and_then(serde_json::Value::as_array).is_some_and(|a| a.is_empty()) { fields.remove(key); }
                }
            }
        }
    }
}

pub use policy::{evaluate_policy, validate_policy, validate_scope};
pub use path::{evaluate_path, evaluate_return, explore_path, explore_return};
pub use network::{evaluate_network, explore_network};
