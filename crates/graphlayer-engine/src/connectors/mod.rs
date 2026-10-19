//! Rust implementations of production data-source connectors.
//!
//! The Python compatibility layer forwards complete requests here. All parsing,
//! filtering, pagination, normalization and correlation decisions live in Rust.

mod common;
mod netbox;
mod servicenow;
mod helpers;
mod ssh;
mod solarwinds;
mod fileshare;
mod api;
mod appid;

#[cfg(test)]
#[path = "../connectors_netbox.rs"]
mod connectors_netbox_tests;

#[cfg(test)]
#[path = "../connectors_servicenow.rs"]
mod connectors_servicenow_tests;

use serde_json::Value;

pub fn dispatch(operation: &str, input: Value) -> Result<Value, String> {
    if operation.starts_with("ssh.") {
        return ssh::dispatch(operation, &input);
    }
    if operation.starts_with("solarwinds.") {
        return solarwinds::dispatch(operation, &input);
    }
    if operation.starts_with("fileshare.") {
        return fileshare::dispatch(operation, &input);
    }
    if operation.starts_with("api.") {
        return api::dispatch(operation, &input);
    }
    if operation.starts_with("appid.") {
        return appid::dispatch(operation, &input);
    }
    if operation.starts_with("connector.") {
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
        "servicenow.load_state" => servicenow::load_state(&input),
        "servicenow.save_state" => servicenow::save_state(&input),
        "servicenow.resolve_updated_since" => servicenow::resolve_updated_since(&input),
        _ => Err("unsupported_operation".to_string()),
    }
}

pub struct SessionDispatcher {
    inner: ssh::SessionDispatcher,
    api: api::SessionDispatcher,
}

impl SessionDispatcher {
    pub fn new() -> Self { Self { inner: ssh::SessionDispatcher::new(), api: api::SessionDispatcher::new() } }

    pub fn dispatch(&mut self, operation: &str, input: Value) -> Result<Value, String> {
        if operation.starts_with("ssh.") { self.inner.dispatch(operation, input) }
        else if operation.starts_with("api.") { self.api.dispatch(operation, input) }
        else { dispatch(operation, input) }
    }
}

impl Default for SessionDispatcher {
    fn default() -> Self { Self::new() }
}
