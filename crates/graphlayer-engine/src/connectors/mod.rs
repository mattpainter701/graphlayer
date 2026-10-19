//! Rust implementations of production data-source connectors.
//!
//! The Python compatibility layer forwards complete requests here. All parsing,
//! filtering, pagination, normalization and correlation decisions live in Rust.

mod common;
mod netbox;
mod servicenow;
mod helpers;

#[cfg(test)]
#[path = "../connectors_netbox.rs"]
mod connectors_netbox_tests;

#[cfg(test)]
#[path = "../connectors_servicenow.rs"]
mod connectors_servicenow_tests;

use serde_json::Value;

pub fn dispatch(operation: &str, input: Value) -> Result<Value, String> {
    if operation.starts_with("connector.") || operation.starts_with("fileshare.") || operation == "solarwinds.sanitize_filename" {
        return helpers::dispatch(operation, &input);
    }
    match operation {
        "netbox.normalize" => netbox::normalize(&input),
        "netbox.collect" => netbox::collect(&input),
        "netbox.ping" => netbox::ping(&input),
        "netbox.fetch_endpoint_page" => netbox::fetch_endpoint_page(&input),
        "netbox.fetch_endpoint" => netbox::fetch_endpoint(&input),
        "servicenow.normalize" => servicenow::normalize(&input),
        "servicenow.collect" => servicenow::collect(&input),
        "servicenow.ping" => servicenow::ping(&input),
        "servicenow.fetch_table_page" => servicenow::fetch_table_page(&input),
        "servicenow.fetch_table" => servicenow::fetch_table(&input),
        _ => Err("unsupported_operation".to_string()),
    }
}
