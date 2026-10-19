use serde_json::Value;
use sha2::{Digest, Sha256};
use chrono::{DateTime, NaiveDateTime, SecondsFormat, Utc};

pub const MAX_BODY_BYTES: usize = 16 * 1024 * 1024;
pub const MAX_PAGES: usize = 100;
pub const MAX_PAGE_SIZE: usize = 1000;
pub const MAX_TOTAL_RESPONSE_BYTES: usize = 128 * 1024 * 1024;
pub const MAX_TOTAL_RECORDS: usize = 100_000;

pub fn text(value: &Value) -> String {
    match value {
        Value::Null => String::new(),
        Value::String(s) => s.clone(),
        Value::Number(n) => n.to_string(),
        Value::Bool(b) => b.to_string(),
        Value::Array(_) | Value::Object(_) => String::new(),
    }
}

pub fn value(record: &Value, key: &str) -> String {
    let direct = record.get(key).unwrap_or(&Value::Null);
    if direct.is_object() {
        for candidate in ["display", "display_value", "displayValue", "name", "slug", "value", "address", "id", "sys_id"] {
            let item = direct.get(candidate).unwrap_or(&Value::Null);
            if !item.is_null() && !text(item).is_empty() { return text(item); }
        }
        String::new()
    } else if !direct.is_null() {
        text(direct)
    } else {
        text(record.get(format!("{key}.display_value")).unwrap_or(&Value::Null))
    }
}

pub fn raw_value(record: &Value, key: &str) -> Value {
    record.get(key).cloned().unwrap_or(Value::Null)
}

pub fn nested(record: &Value, keys: &[&str]) -> String {
    let mut at = record;
    for key in keys {
        at = match at.get(*key) { Some(v) => v, None => return String::new() };
    }
    if at.is_object() {
        for key in ["display", "display_value", "displayValue", "name", "slug", "value", "address", "id", "sys_id"] {
            let item = at.get(key).unwrap_or(&Value::Null);
            if !item.is_null() && !text(item).is_empty() { return text(item); }
        }
        String::new()
    } else { text(at) }
}

pub fn service_value(record: &Value, key: &str) -> String {
    let direct = record.get(key).unwrap_or(&Value::Null);
    if direct.is_object() {
        for candidate in ["display_value", "displayValue", "value", "sys_id", "id"] {
            let item = direct.get(candidate).unwrap_or(&Value::Null);
            if !item.is_null() && !text(item).is_empty() { return text(item); }
        }
        String::new()
    } else if !direct.is_null() {
        text(direct)
    } else {
        for candidate in [format!("{key}.display_value"), format!("{key}_display_value")] {
            let found = record.get(candidate).unwrap_or(&Value::Null);
            if !found.is_null() && !text(found).is_empty() { return text(found); }
        }
        String::new()
    }
}

pub fn correlation(pairs: &[(&str, String)]) -> Value {
    let mut seen = std::collections::HashSet::new();
    let mut result = Vec::new();
    for (label, raw) in pairs {
        let val = raw.trim().to_lowercase();
        let label = label.trim().to_lowercase();
        if !val.is_empty() && !label.is_empty() {
            let key = format!("{label}:{val}");
            if seen.insert(key.clone()) { result.push(Value::String(key)); }
        }
    }
    Value::Array(result)
}

pub fn record(kind: &str, connector: &str, id: String, payload: Value, confidence: f64, metadata: Value) -> Value {
    let bytes = serde_json::to_vec(&payload).unwrap_or_default();
    let hash = Sha256::digest(bytes);
    let fingerprint = format!("sha256:{hash:x}");
    serde_json::json!({
        "record_type": kind,
        "source_connector": connector,
        "source_id": id,
        "payload": payload,
        "confidence": confidence,
        "collected_at": now_rfc3339(),
        "raw_fingerprint": fingerprint,
        "metadata": metadata,
    })
}

fn now_rfc3339() -> String {
    Utc::now().to_rfc3339_opts(SecondsFormat::AutoSi, false)
}

pub fn latest_timestamp(values: impl IntoIterator<Item = String>, fallback: Option<String>) -> String {
    let parsed = values.into_iter().filter_map(|s| parse_timestamp(&s)).max();
    parsed.or_else(|| fallback.as_deref().and_then(parse_timestamp)).map(|dt| dt.to_rfc3339_opts(SecondsFormat::AutoSi, false)).unwrap_or_else(now_rfc3339)
}

fn parse_timestamp(value: &str) -> Option<DateTime<Utc>> {
    let trimmed = value.trim();
    if trimmed.is_empty() { return None; }
    if let Ok(dt) = DateTime::parse_from_rfc3339(trimmed) { return Some(dt.with_timezone(&Utc)); }
    for fmt in ["%Y-%m-%d %H:%M:%S%.f", "%Y-%m-%dT%H:%M:%S%.f"] {
        if let Ok(dt) = NaiveDateTime::parse_from_str(trimmed, fmt) { return Some(dt.and_utc()); }
    }
    None
}

pub fn normalize_base_url(raw: &Value, error: &str) -> Result<url::Url, String> {
    let mut input = text(raw).trim().to_string();
    if input.is_empty() { return Err(error.to_string()); }
    if !input.starts_with("http://") && !input.starts_with("https://") { input = format!("https://{input}"); }
    let url = url::Url::parse(&input).map_err(|_| "invalid_target_url".to_string())?;
    if !matches!(url.scheme(), "http" | "https") || url.host_str().is_none() || url.username() != "" || url.password().is_some() || url.fragment().is_some() {
        return Err("invalid_target_url".to_string());
    }
    Ok(url)
}
