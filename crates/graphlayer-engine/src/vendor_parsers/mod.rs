mod fortios;
mod panos;

pub fn parse_panos(input: &str) -> Result<serde_json::Value, String> { panos::parse_with_hash(input, &common::sha256(input.as_bytes())) }
pub fn parse_panos_with_hash(input: &str, artifact_sha256: &str) -> Result<serde_json::Value, String> { panos::parse_with_hash(input, artifact_sha256) }
pub fn parse_fortigate(input: &str) -> Result<serde_json::Value, String> { fortios::parse_with_hash(input, &common::sha256(input.as_bytes())) }
pub fn parse_fortigate_with_hash(input: &str, artifact_sha256: &str) -> Result<serde_json::Value, String> { fortios::parse_with_hash(input, artifact_sha256) }

mod common;
