use super::common::{canonical_fingerprint, record, text};
use super::fileshare::{prepare_private_output_directory, secure_write};
use chrono::{DateTime, Duration, Utc};
use serde_json::{json, Value};
use std::{
    collections::{BTreeMap, BTreeSet},
    path::{Component, Path},
};

const MAX_CONFIG_FILE_BYTES: usize = 16 * 1024 * 1024;

pub fn dispatch(operation: &str, input: &Value) -> Result<Value, String> {
    match operation {
        "connector.fingerprint" => Ok(json!(canonical_fingerprint(
            input.get("data").unwrap_or(&Value::Null)
        ))),
        "connector.correlation_keys" => Ok(correlation_keys(
            input
                .get("pairs")
                .and_then(Value::as_array)
                .map(Vec::as_slice)
                .unwrap_or(&[]),
        )),
        "connector.build_record" => Ok(build_record(input)),
        "connector.deduplicate" => Ok(json!(deduplicate(
            input
                .get("records")
                .and_then(Value::as_array)
                .map(Vec::as_slice)
                .unwrap_or(&[])
        ))),
        "connector.correlation_report" => Ok(correlation_report(
            input
                .get("records")
                .and_then(Value::as_array)
                .map(Vec::as_slice)
                .unwrap_or(&[]),
        )),
        "connector.filter_confidence" => filter_records(input),
        "connector.staleness_decay" => staleness(input),
        "connector.build_payload" => build_payload(input),
        "connector.delta_sync" => delta_sync(input),
        "connector.delta_select" => delta_select(input),
        "connector.dedup_key" => Ok(json!(record_key(
            input.get("record").unwrap_or(&Value::Null)
        ))),
        "connector.normalize_hostname" => Ok(json!(text(&input["hostname"]).trim().to_lowercase())),
        "connector.normalize_correlation_value" => {
            Ok(json!(text(&input["value"]).trim().to_lowercase()))
        }
        "connector.validate_required_fields" => validate_fields(input),
        "connector.normalize_path" => Ok(json!(normalize_path(&text(&input["path"])))),
        "connector.prepare_output_directory" => prepare_output_directory(input),
        "connector.write_config" => write_config(input),
        "fileshare.normalize_path" => Ok(json!(normalize_path(&text(&input["path"])))),
        "fileshare.create_local_filename" => Ok(json!(local_filename(
            &text(&input["remote_path"]),
            &text(&input["root_path"])
        ))),
        "fileshare.matches_patterns" => matches_patterns(input),
        "solarwinds.sanitize_filename" => Ok(json!(sanitize_filename(&text(&input["name"])))),
        "connector.normalize_type" => Ok(json!(normalize_type(&text(&input["connector_type"])))),
        _ => Err("unsupported_operation".into()),
    }
}

fn prepare_output_directory(input: &Value) -> Result<Value, String> {
    let path = text(&input["path"]);
    if path.len() > 4096 {
        return Err("invalid_output_directory".into());
    }
    let canonical = prepare_private_output_directory(Path::new(&path))?;
    Ok(json!({"path":canonical.to_string_lossy()}))
}

fn write_config(input: &Value) -> Result<Value, String> {
    let root = text(&input["working_folder"]);
    let file_name = text(&input["file_name"]);
    let content = input
        .get("content")
        .and_then(Value::as_str)
        .ok_or_else(|| "invalid_config_content".to_string())?;
    let path = Path::new(&file_name);
    if root.is_empty()
        || root.len() > 4096
        || file_name.is_empty()
        || file_name.len() > 255
        || path.components().count() != 1
        || !matches!(path.components().next(), Some(Component::Normal(_)))
        || file_name.ends_with(' ')
        || file_name.ends_with('.')
        || file_name
            .chars()
            .any(|ch| ch.is_control() || "<>:\"/\\|?*".contains(ch))
    {
        return Err("invalid_output_path".into());
    }
    let bytes = content.as_bytes();
    if bytes.len() > MAX_CONFIG_FILE_BYTES {
        return Err("config_file_too_large".into());
    }
    let canonical_root = prepare_private_output_directory(Path::new(&root))?;
    secure_write(&canonical_root, &file_name, bytes)
        .map_err(|_| "config_file_write_failed".to_string())?;
    Ok(json!({"path":canonical_root.join(file_name).to_string_lossy()}))
}

fn normalize_type(v: &str) -> String {
    v.trim().to_lowercase().replace('-', "_")
}
fn correlation_keys(pairs: &[Value]) -> Value {
    let mut seen = BTreeSet::new();
    let mut out = Vec::new();
    for pair in pairs {
        let Some(a) = pair.as_array() else { continue };
        if a.len() != 2 {
            continue;
        }
        let label = text(&a[0]).trim().to_lowercase();
        let value = text(&a[1]).trim().to_lowercase();
        if !label.is_empty() && !value.is_empty() {
            let key = format!("{label}:{value}");
            if seen.insert(key.clone()) {
                out.push(json!(key));
            }
        }
    }
    Value::Array(out)
}
fn record_key(r: &Value) -> String {
    let fp = text(&r["raw_fingerprint"]);
    if !fp.is_empty() {
        return format!("fingerprint:{fp}");
    }
    let mut keys = r["payload"]["correlation_keys"]
        .as_array()
        .cloned()
        .unwrap_or_default()
        .iter()
        .map(|v| text(v).trim().to_lowercase())
        .filter(|s| !s.is_empty())
        .collect::<Vec<_>>();
    keys.sort();
    keys.dedup();
    if !keys.is_empty() {
        return format!("correlation:{}", keys.join("|"));
    }
    format!(
        "source:{}:{}:{}",
        text(&r["source_connector"]),
        text(&r["record_type"]),
        text(&r["source_id"])
    )
}
fn deduplicate(records: &[Value]) -> Vec<Value> {
    let mut order = Vec::<String>::new();
    let mut winners = BTreeMap::<String, Value>::new();
    for r in records {
        let k = record_key(r);
        if let Some(old) = winners.get(&k) {
            if r["confidence"].as_f64().unwrap_or(0.0) > old["confidence"].as_f64().unwrap_or(0.0) {
                winners.insert(k, r.clone());
            }
        } else {
            order.push(k.clone());
            winners.insert(k, r.clone());
        }
    }
    order
        .iter()
        .filter_map(|k| winners.get(k).cloned())
        .collect()
}
fn correlation_report(records: &[Value]) -> Value {
    let mut groups = BTreeMap::<String, Vec<Value>>::new();
    let mut order = Vec::<String>::new();
    for r in records {
        let key = record_key(r);
        if !groups.contains_key(&key) {
            order.push(key.clone());
        }
        groups.entry(key).or_default().push(r.clone());
    }
    let winners = deduplicate(records)
        .into_iter()
        .map(|r| (record_key(&r), r))
        .collect::<BTreeMap<_, _>>();
    let mut output = Vec::new();
    for key in &order {
        let rs = &groups[key];
        let confidences = rs
            .iter()
            .map(|r| r["confidence"].as_f64().unwrap_or(0.0))
            .collect::<Vec<_>>();
        let mut correlation = BTreeSet::new();
        for r in rs {
            if let Some(keys) = r["payload"]["correlation_keys"].as_array() {
                for k in keys {
                    let s = text(k).trim().to_lowercase();
                    if !s.is_empty() {
                        correlation.insert(s);
                    }
                }
            }
        }
        let sum = confidences.iter().sum::<f64>();
        let mean = (sum / confidences.len() as f64 * 1000.0).round() / 1000.0;
        let winner = &winners[key];
        output.push(json!({"dedup_key":key,"record_count":rs.len(),"source_ids":rs.iter().map(|r|r["source_id"].clone()).collect::<Vec<_>>(),"source_connectors":rs.iter().map(|r|r["source_connector"].clone()).collect::<Vec<_>>(),"correlation_keys":correlation,"confidence":{"min":confidences.iter().copied().fold(f64::INFINITY,f64::min),"max":confidences.iter().copied().fold(f64::NEG_INFINITY,f64::max),"mean":mean},"selected_source_id":winner["source_id"],"selected_confidence":winner["confidence"]}));
    }
    json!({"total_records":records.len(),"unique_groups":output.len(),"duplicate_groups":output.iter().filter(|g|g["record_count"].as_u64().unwrap_or(0)>1).count(),"groups":output})
}
fn build_record(input: &Value) -> Value {
    let payload = input.get("payload").cloned().unwrap_or_else(|| json!({}));
    record(
        &text(&input["record_type"]),
        &text(&input["source_connector"]),
        text(&input["source_id"]),
        payload,
        input["confidence"].as_f64().unwrap_or(1.0),
        input.get("metadata").cloned().unwrap_or_else(|| json!({})),
    )
}
fn filter_records(input: &Value) -> Result<Value, String> {
    let threshold = input["minimum_confidence"]
        .as_f64()
        .ok_or_else(|| "invalid_confidence_threshold".to_string())?;
    if !threshold.is_finite() || !(0.0..=1.0).contains(&threshold) {
        return Err("minimum_confidence must be between 0.0 and 1.0".into());
    }
    Ok(Value::Array(
        input["records"]
            .as_array()
            .cloned()
            .unwrap_or_default()
            .into_iter()
            .filter(|r| r["confidence"].as_f64().unwrap_or(0.0) >= threshold)
            .collect(),
    ))
}
fn staleness(input: &Value) -> Result<Value, String> {
    let half = input["half_life_days"].as_f64().unwrap_or(30.0);
    if !half.is_finite() || half <= 0.0 {
        return Err("half_life_days must be greater than 0".into());
    }
    let confidence = input["confidence"].as_f64().unwrap_or(0.0);
    let collected = text(&input["collected_at"]);
    if collected.is_empty() {
        return Ok(json!(confidence.clamp(0.0, 1.0)));
    }
    let parsed = DateTime::parse_from_rfc3339(&collected)
        .map_err(|_| "invalid collected_at timestamp".to_string())?
        .with_timezone(&Utc);
    let now = if let Some(s) = input.get("now").filter(|v| !text(v).is_empty()).map(text) {
        DateTime::parse_from_rfc3339(&s)
            .map_err(|_| "invalid now timestamp".to_string())?
            .with_timezone(&Utc)
    } else {
        Utc::now()
    };
    let age = (now - parsed).max(Duration::zero()).num_milliseconds() as f64 / 86_400_000.0;
    Ok(json!(
        (confidence * 0.5_f64.powf(age / half)).clamp(0.0, 1.0)
    ))
}
fn build_payload(input: &Value) -> Result<Value, String> {
    let connector = text(&input["source_connector"]);
    let kind = text(&input["record_type"]);
    let confidence = input["confidence"].as_f64().unwrap_or(1.0);
    let metadata = input.get("metadata").cloned().unwrap_or_else(|| json!({}));
    let records=input["records"].as_array().cloned().unwrap_or_default().into_iter().enumerate().map(|(i,r)|build_record(&json!({"record_type":kind,"source_connector":connector,"source_id":r.get("id").filter(|v|!text(v).is_empty()).or_else(||r.get("name")).map(text).unwrap_or_else(||i.to_string()),"payload":r,"confidence":confidence,"metadata":metadata}))).collect::<Vec<_>>();
    let filtered = if input
        .get("minimum_confidence")
        .is_some_and(|v| !v.is_null())
    {
        filter_records(
            &json!({"minimum_confidence":input["minimum_confidence"],"records":records}),
        )?
        .as_array()
        .cloned()
        .unwrap_or_default()
    } else {
        records
    };
    Ok(
        json!({"record_type":kind,"connector":connector,"count":filtered.len(),"minimum_confidence":input.get("minimum_confidence").cloned().unwrap_or(Value::Null),"records":filtered}),
    )
}
fn delta_sync(input: &Value) -> Result<Value, String> {
    let records = input["records"].as_array().cloned().unwrap_or_default();
    let prior = input
        .get("previous_checkpoint")
        .cloned()
        .unwrap_or_else(|| json!({}));
    let fingerprints = prior["fingerprints"]
        .as_object()
        .cloned()
        .unwrap_or_default();
    let mut current = BTreeMap::<String, String>::new();
    let mut deltas = Vec::new();
    for r in &records {
        let id = text(&r["source_id"]);
        let fp = text(&r["raw_fingerprint"]);
        current.insert(id.clone(), fp.clone());
        let previous = fingerprints.get(&id).map(text);
        let action = match previous.as_ref() {
            None => "added",
            Some(old) if old == &fp => "unchanged",
            Some(_) => "updated",
        };
        deltas.push(json!({"action":action,"record":r,"previous_fingerprint":previous}));
    }
    if input
        .get("include_stale")
        .and_then(Value::as_bool)
        .unwrap_or(true)
    {
        let ids = current.keys().cloned().collect::<BTreeSet<_>>();
        for (id, fp) in fingerprints {
            if ids.contains(&id) {
                continue;
            }
            let rec = record(
                "stale",
                &config_type(input),
                id.clone(),
                json!({"stale":true,"last_fingerprint":fp}),
                0.0,
                json!({"delta_action":"stale"}),
            );
            let mut rec = rec;
            rec["raw_fingerprint"] = json!(fp);
            deltas.push(json!({"action":"stale","record":rec,"previous_fingerprint":fp}));
        }
    }
    let next = json!({"connector":config_type(input),"high_watermark":input.get("high_watermark").filter(|v|!v.is_null()).cloned().unwrap_or(prior["high_watermark"].clone()),"fingerprints":current,"source_ids":records.iter().map(|r|text(&r["source_id"])).collect::<BTreeSet<_>>(),"updated_at":Utc::now().to_rfc3339_opts(chrono::SecondsFormat::AutoSi,false)});
    Ok(json!({"records":deltas,"checkpoint":next}))
}
fn delta_select(input: &Value) -> Result<Value, String> {
    let records = input["records"].as_array().cloned().unwrap_or_default();
    let actions = ["added", "updated", "unchanged", "stale"];
    let mut selected = serde_json::Map::new();
    let mut summary = serde_json::Map::new();
    for action in actions {
        let rows = records
            .iter()
            .filter(|r| r["action"].as_str() == Some(action))
            .cloned()
            .collect::<Vec<_>>();
        summary.insert(action.into(), json!(rows.len()));
        selected.insert(action.into(), json!(rows));
    }
    summary.insert("total".into(), json!(records.len()));
    Ok(json!({"selected":selected,"summary":summary}))
}
fn config_type(i: &Value) -> String {
    let s = text(&i["source_connector"]);
    if s.is_empty() {
        "unknown".into()
    } else {
        s
    }
}
fn validate_fields(i: &Value) -> Result<Value, String> {
    let mut missing = Vec::new();
    if let Some(fields) = i["fields"].as_array() {
        for field in fields {
            let name = text(field);
            if i.get("values")
                .and_then(|v| v.get(&name))
                .is_none_or(|v| v.is_null() || text(v).is_empty())
            {
                missing.push(name);
            }
        }
    }
    if missing.is_empty() {
        Ok(json!({"valid":true}))
    } else {
        Err(format!(
            "missing required connector fields: {}",
            missing.join(", ")
        ))
    }
}
fn normalize_path(raw: &str) -> String {
    let path = raw.trim();
    if path.is_empty() || path == "/" {
        return String::new();
    }
    let mut result = path.to_string();
    if result.ends_with('/') {
        result.pop();
    }
    if !result.starts_with('/') {
        result.insert(0, '/');
    }
    result
}
fn local_filename(remote: &str, root: &str) -> String {
    let relative = remote
        .strip_prefix(root)
        .unwrap_or(remote)
        .trim_start_matches('/')
        .replace('\\', "/");
    relative
}
fn matches_patterns(i: &Value) -> Result<Value, String> {
    let name = text(&i["filename"]);
    let includes = i["file_patterns"].as_array().cloned().unwrap_or_default();
    let excludes = i["exclude_patterns"]
        .as_array()
        .cloned()
        .unwrap_or_default();
    let included = if includes.is_empty() {
        true
    } else {
        pattern_match(&name, &includes)?
    };
    let excluded = pattern_match(&name, &excludes)?;
    Ok(json!(included && !excluded))
}
fn pattern_match(name: &str, patterns: &[Value]) -> Result<bool, String> {
    for p in patterns {
        let pat = text(p);
        if pat.is_empty() {
            continue;
        }
        let regex = regex::Regex::new(&pat).map_err(|_| "invalid_file_pattern".to_string())?;
        if regex.is_match(name) {
            return Ok(true);
        }
    }
    Ok(false)
}
fn sanitize_filename(name: &str) -> String {
    name.replace(' ', "_")
        .chars()
        .filter(|c| !"<>:\"/\\|?*".contains(*c))
        .take(200)
        .collect()
}

#[cfg(test)]
mod output_tests {
    use super::*;
    use std::fs;

    fn temp_dir(label: &str) -> std::path::PathBuf {
        let path = std::env::temp_dir().join(format!(
            "graphlayer-config-output-{label}-{}-{}",
            std::process::id(),
            std::time::SystemTime::now()
                .duration_since(std::time::UNIX_EPOCH)
                .unwrap()
                .as_nanos()
        ));
        fs::create_dir_all(&path).unwrap();
        path
    }

    #[test]
    fn output_operations_create_private_directory_and_write_content() {
        let parent = temp_dir("private");
        let requested = parent.join("new/nested");
        let prepared = dispatch(
            "connector.prepare_output_directory",
            &json!({"path":requested.to_string_lossy()}),
        )
        .unwrap();
        let root = Path::new(prepared["path"].as_str().unwrap());
        let result = dispatch(
            "connector.write_config",
            &json!({"working_folder":root.to_string_lossy(),"file_name":"router_running.txt","content":"sensitive config"}),
        )
        .unwrap();
        assert_eq!(
            fs::read(result["path"].as_str().unwrap()).unwrap(),
            b"sensitive config"
        );
        let replaced = dispatch(
            "connector.write_config",
            &json!({"working_folder":root.to_string_lossy(),"file_name":"router_running.txt","content":"updated config"}),
        )
        .unwrap();
        assert_eq!(
            fs::read(replaced["path"].as_str().unwrap()).unwrap(),
            b"updated config"
        );
        #[cfg(unix)]
        {
            use std::os::unix::fs::PermissionsExt;
            assert_eq!(
                fs::metadata(root).unwrap().permissions().mode() & 0o777,
                0o700
            );
            assert_eq!(
                fs::metadata(result["path"].as_str().unwrap())
                    .unwrap()
                    .permissions()
                    .mode()
                    & 0o777,
                0o600
            );
        }
        let _ = fs::remove_dir_all(parent);
    }

    #[test]
    fn config_writer_rejects_traversal_and_oversized_content() {
        let root = temp_dir("validation");
        let traversal = dispatch(
            "connector.write_config",
            &json!({"working_folder":root.to_string_lossy(),"file_name":"../escape.txt","content":"x"}),
        );
        assert_eq!(traversal.unwrap_err(), "invalid_output_path");
        let too_large = "x".repeat(MAX_CONFIG_FILE_BYTES + 1);
        let oversized = dispatch(
            "connector.write_config",
            &json!({"working_folder":root.to_string_lossy(),"file_name":"large.txt","content":too_large}),
        );
        assert_eq!(oversized.unwrap_err(), "config_file_too_large");
        let _ = fs::remove_dir_all(root);
    }

    #[test]
    fn explicit_output_root_may_use_parent_components() {
        let parent = temp_dir("parent-root");
        let selected_name = format!("{}-selected", parent.file_name().unwrap().to_string_lossy());
        let selected = parent.join("..").join(selected_name);
        let result = dispatch(
            "connector.prepare_output_directory",
            &json!({"path":selected.to_string_lossy()}),
        )
        .unwrap();
        let canonical = Path::new(result["path"].as_str().unwrap());
        assert!(canonical.is_dir());
        let _ = fs::remove_dir_all(canonical);
        let _ = fs::remove_dir_all(parent);
    }

    #[cfg(unix)]
    #[test]
    fn config_writer_rejects_symlink_target_without_clobbering_it() {
        use std::os::unix::fs::symlink;
        let root = temp_dir("symlink");
        let outside = root.join("outside.txt");
        let link = root.join("config.txt");
        fs::write(&outside, b"original").unwrap();
        symlink(&outside, &link).unwrap();
        let result = dispatch(
            "connector.write_config",
            &json!({"working_folder":root.to_string_lossy(),"file_name":"config.txt","content":"replacement"}),
        );
        assert_eq!(result.unwrap_err(), "config_file_write_failed");
        assert_eq!(fs::read(outside).unwrap(), b"original");
        let _ = fs::remove_dir_all(root);
    }

    #[cfg(unix)]
    #[test]
    fn output_directory_rejects_symlink_components() {
        use std::os::unix::fs::symlink;
        let root = temp_dir("directory-link");
        let outside = temp_dir("directory-outside");
        symlink(&outside, root.join("link")).unwrap();
        let result = dispatch(
            "connector.prepare_output_directory",
            &json!({"path":root.join("link/child").to_string_lossy()}),
        );
        assert_eq!(result.unwrap_err(), "invalid_output_directory");
        let _ = fs::remove_dir_all(root);
        let _ = fs::remove_dir_all(outside);
    }
}
