use base64::Engine;
use graphlayer_engine::dispatch::{dispatch, source_text};
use serde_json::json;
use sha2::{Digest, Sha256};

#[test]
fn raw_upload_identity_survives_crlf_and_invalid_utf8() {
    let bytes = b"hostname branch-fw\r\n!\xff\xe2\x82\r\naccess-list IN extended permit tcp any any eq 443\r\naccess-group IN in interface inside\r\n";
    let hash = format!("{:x}", Sha256::digest(bytes));
    let encoded = base64::engine::general_purpose::STANDARD.encode(bytes);
    let config = dispatch("parse_asa", json!({"config_base64":encoded})).unwrap();
    assert_eq!(config["artifact_sha256"], hash);
    assert_eq!(config["source_lines"][2]["number"], 3);
    assert!(config["source_lines"][2]["text"].as_str().unwrap().contains("access-list IN"));
    // The JSON representation follows the released Go per-invalid-byte replacement.
    assert_eq!(source_text(b"\xff\xe2\x82"), "\u{fffd}\u{fffd}\u{fffd}");
    let evidence = config["evidence"].as_array().unwrap();
    let rule = &config["chains"][0]["rules"][0];
    for reference in rule["evidence_refs"].as_array().unwrap() {
        assert!(evidence.iter().any(|item| item["id"] == *reference));
    }
    assert_ne!(hash, format!("{:x}", Sha256::digest(source_text(bytes).as_bytes())));
}

#[test]
fn oversized_auto_detection_is_rejected_before_decoding() {
    let oversized = "A".repeat(((2 << 20) + 2) / 3 * 4 + 4);
    assert_eq!(dispatch("parse_config", json!({"vendor":"auto","config_base64":oversized})).unwrap_err(), "configuration exceeds input budget");
}
