use serde_json::{json, Value};
use std::collections::{HashMap, HashSet};
use std::net::{IpAddr, Ipv4Addr};
use std::str::FromStr;

fn config(input: &Value) -> &Value {
    input.get("config").unwrap_or(input)
}
fn s<'a>(v: &'a Value, k: &str) -> &'a str {
    v.get(k).and_then(Value::as_str).unwrap_or("")
}
fn arr<'a>(v: &'a Value, k: &str) -> &'a [Value] {
    v.get(k)
        .and_then(Value::as_array)
        .map(Vec::as_slice)
        .unwrap_or(&[])
}
fn strings(v: &Value, k: &str) -> Vec<String> {
    arr(v, k)
        .iter()
        .filter_map(Value::as_str)
        .map(str::to_owned)
        .collect()
}
fn family(id: &str) -> &'static str {
    let Some((kind, name)) = id.split_once(':') else {
        return "";
    };
    if name.trim().is_empty() {
        return "";
    }
    match kind {
        "network_object" | "network_group" => "network",
        "service_object" | "service_group" => "service",
        _ => "",
    }
}
fn normalized_port(v: &str) -> bool {
    if v.is_empty() {
        return true;
    }
    let p: Vec<_> = v.split('-').collect();
    if p.len() > 2 {
        return false;
    }
    let Ok(lo) = p[0].parse::<i64>() else {
        return false;
    };
    if !(1..=65535).contains(&lo) {
        return false;
    }
    if p.len() == 1 {
        return true;
    }
    let Ok(hi) = p[1].parse::<i64>() else {
        return false;
    };
    hi >= lo && hi <= 65535
}
fn native_address(v: &str) -> bool {
    if let Ok(p) = v.parse::<std::net::IpAddr>() {
        return !matches!(p,IpAddr::V6(x) if x.to_ipv4_mapped().is_some());
    }
    if let Some((a, b)) = v.split_once('/') {
        let Ok(ip) = IpAddr::from_str(a) else {
            return false;
        };
        let Ok(bits) = b.parse::<u8>() else {
            return false;
        };
        let max = if ip.is_ipv4() { 32 } else { 128 };
        if bits > max || matches!(ip,IpAddr::V6(x) if x.to_ipv4_mapped().is_some()) {
            return false;
        }
        let mask = if ip.is_ipv4() {
            let n = u32::from(match ip {
                IpAddr::V4(a) => a,
                _ => Ipv4Addr::UNSPECIFIED,
            });
            let m = if bits == 0 {
                0
            } else {
                u32::MAX << (32 - bits)
            };
            IpAddr::V4(Ipv4Addr::from(n & m))
        } else {
            let IpAddr::V6(a) = ip else { unreachable!() };
            let n = u128::from(a);
            let m = if bits == 0 {
                0
            } else {
                u128::MAX << (128 - bits)
            };
            IpAddr::V6(std::net::Ipv6Addr::from(n & m))
        };
        return mask == ip;
    }
    false
}
fn refs_exist(refs: &[String], evidence: &HashSet<String>) -> bool {
    refs.iter().all(|r| evidence.contains(r))
}

pub fn validate_inventory(input: Value) -> Result<Value, String> {
    let c = config(&input);
    let version = s(c, "parser_version");
    if version == "asa-config-v1" {
        if !arr(c, "objects").is_empty() || !arr(c, "rule_origins").is_empty() {
            return Err("legacy snapshot cannot claim a new object inventory".into());
        }
        return Ok(json!({"valid":true}));
    }
    let vendor = s(c, "vendor");
    if !((vendor == "cisco-asa"
        && matches!(version, "asa-config-v1" | "asa-config-v2" | "asa-config-v3"))
        || (vendor == "paloalto-panos" && version == "panos-config-v1")
        || (vendor == "fortinet-fortigate" && version == "fortios-config-v1"))
    {
        return Err("unsupported snapshot parser version".into());
    }
    let evidence: HashSet<String> = arr(c, "evidence")
        .iter()
        .map(|e| s(e, "id").to_owned())
        .collect();
    let mut objects: HashMap<String, Value> = HashMap::new();
    let mut ids = Vec::new();
    for o in arr(c, "objects") {
        let id = s(o, "id");
        let kind = s(o, "kind");
        let name = s(o, "name");
        let f = family(id);
        if f.is_empty()
            || id != format!("{}:{}", kind, name)
            || objects.contains_key(id)
            || strings(o, "evidence_refs").is_empty()
            || !refs_exist(&strings(o, "evidence_refs"), &evidence)
        {
            return Err("invalid object identity or evidence".into());
        }
        let members = arr(o, "members");
        let resolved = o.get("resolved").and_then(Value::as_bool).unwrap_or(false);
        if resolved && members.is_empty() {
            return Err("resolved object has no members".into());
        }
        for m in members {
            if !refs_exist(&strings(m, "evidence_refs"), &evidence) {
                return Err("missing member evidence".into());
            }
            match s(m, "kind") {
                "reference" => {
                    if family(s(m, "object_id")) != f
                        || !s(m, "value").is_empty()
                        || !s(m, "protocol").is_empty()
                        || !s(m, "source_port").is_empty()
                        || !s(m, "destination_port").is_empty()
                    {
                        return Err("object dependency crosses namespaces".into());
                    }
                }
                "address" => {
                    if f != "network" || !native_address(s(m, "value")) {
                        return Err("invalid address member".into());
                    }
                }
                "service" => {
                    if f != "service"
                        || !matches!(s(m, "protocol"), "tcp" | "udp" | "tcp-udp")
                        || !normalized_port(s(m, "source_port"))
                        || !normalized_port(s(m, "destination_port"))
                    {
                        return Err("invalid service member".into());
                    }
                }
                "unsupported" => {
                    let label = s(m, "value");
                    if resolved
                        || !(label.is_empty()
                            || matches!(
                                label,
                                "unsupported network member"
                                    | "unsupported service object member"
                                    | "unsupported service member"
                            ))
                        || !s(m, "object_id").is_empty()
                        || !s(m, "protocol").is_empty()
                        || !s(m, "source_port").is_empty()
                        || !s(m, "destination_port").is_empty()
                    {
                        return Err("unsupported member cannot be resolved".into());
                    }
                }
                _ => return Err("unknown object member kind".into()),
            }
        }
        ids.push(id.to_owned());
        objects.insert(id.to_owned(), o.clone());
    }
    let (mut states, mut depths) = (
        HashMap::<String, u8>::new(),
        HashMap::<String, usize>::new(),
    );
    fn visit(
        id: &str,
        level: usize,
        objects: &HashMap<String, Value>,
        states: &mut HashMap<String, u8>,
        depths: &mut HashMap<String, usize>,
    ) -> Result<usize, String> {
        if level > 16 {
            return Err("object dependency depth exceeds the supported budget".into());
        }
        let Some(o) = objects.get(id) else {
            return Err("invalid resolved object dependency".into());
        };
        if !o["resolved"].as_bool().unwrap_or(false) || states.get(id) == Some(&1) {
            return Err("invalid resolved object dependency".into());
        }
        if states.get(id) == Some(&2) {
            let d = depths[id];
            if level + d > 16 {
                return Err("object dependency depth exceeds the supported budget".into());
            }
            return Ok(d);
        }
        states.insert(id.to_owned(), 1);
        let mut depth = 0;
        for m in arr(o, "members") {
            if s(m, "kind") == "reference" {
                let d = visit(s(m, "object_id"), level + 1, objects, states, depths)?;
                depth = depth.max(d + 1)
            }
        }
        if depth > 16 {
            return Err("object dependency depth exceeds the supported budget".into());
        }
        states.insert(id.to_owned(), 2);
        depths.insert(id.to_owned(), depth);
        Ok(depth)
    }
    for id in ids {
        if objects[&id]["resolved"].as_bool().unwrap_or(false) {
            let _ = visit(&id, 0, &objects, &mut states, &mut depths)?;
        }
    }
    let mut rules = HashMap::<String, String>::new();
    for chain in arr(c, "chains") {
        for r in arr(chain, "rules") {
            let id = s(r, "id").to_owned();
            if rules.insert(id, s(chain, "name").to_owned()).is_some() {
                return Err("rule identity reused across chains".into());
            }
        }
    }
    let src_lines = arr(c, "source_lines");
    let mut origins = HashSet::new();
    for o in arr(c, "rule_origins") {
        let id = s(o, "rule_id");
        let line = o.get("line").and_then(Value::as_i64).unwrap_or(0);
        if !rules.contains_key(id)
            || origins.contains(id)
            || rules[id] != s(o, "acl_name")
            || line < 1
            || line as usize > src_lines.len()
        {
            return Err("invalid rule origin".into());
        }
        for x in strings(o, "source_object_ids")
            .into_iter()
            .chain(strings(o, "destination_object_ids"))
        {
            if family(&x) != "network" {
                return Err("invalid address origin namespace".into());
            }
        }
        for x in strings(o, "service_object_ids") {
            if family(&x) != "service" {
                return Err("invalid service origin namespace".into());
            }
        }
        origins.insert(id.to_owned());
    }
    if origins.len() != rules.len() {
        return Err("rule origins do not cover the supplied chains".into());
    }
    Ok(json!({"valid":true}))
}

fn fail(msg: &str) -> Result<Value, String> {
    Err(msg.into())
}
fn ip4_prefix(v: &str) -> Option<(Ipv4Addr, u8)> {
    let (a, b) = v.split_once('/')?;
    let ip = a.parse().ok()?;
    let bits = b.parse::<u8>().ok()?;
    if bits > 32 {
        return None;
    }
    Some((ip, bits))
}
fn p4canonical(v: &str) -> bool {
    let Some((ip, bits)) = ip4_prefix(v) else {
        return false;
    };
    let m = if bits == 0 {
        0
    } else {
        u32::MAX << (32 - bits)
    };
    u32::from(ip) & m == u32::from(ip)
}
fn p4addr(v: &str) -> bool {
    v.parse::<Ipv4Addr>().is_ok()
}
fn refs_ok(v: &Value, key: &str, evidence: &HashSet<String>, required: bool) -> bool {
    let r = strings(v, key);
    (!required || !r.is_empty()) && refs_exist(&r, evidence)
}
fn vendor_match(rule: &Value) -> Result<(), String> {
    let scope = json!({"snapshot_id":"validation","device_id":"validation","context":"validation"});
    let flow = json!({"scope":scope.clone(),"source":"192.0.2.1","destination":"198.51.100.1","protocol":"tcp","source_port":52000,"destination_port":443});
    let bundle = json!({"scope":scope,"complete":false,"default_action":"deny","rules":[rule]});
    crate::investigation_engine::evaluate_policy(json!({"bundle":bundle,"flow":flow})).map(|_| ())
}

pub fn validate_path_config(input: Value) -> Result<Value, String> {
    let c = config(&input);
    let version = s(c, "parser_version");
    let vendor_path = version == "panos-config-v1" || version == "fortios-config-v1";
    if version != "asa-config-v3" && !vendor_path {
        if !c.get("path").unwrap_or(&Value::Null).is_null() {
            return fail("older parser cannot claim path metadata");
        }
        return Ok(json!({"valid":true}));
    }
    let null = Value::Null;
    let p = c.get("path").unwrap_or(&null);
    let want = match s(c, "vendor") {
        "paloalto-panos" => "panos-path-v1",
        "fortinet-fortigate" => "fortios-path-v1",
        _ => "asa-path-v1",
    };
    if p.is_null()
        || s(p, "version") != want
        || arr(p, "interfaces").len() > 1024
        || arr(p, "routes").len() > 10000
        || arr(p, "nat_rules").len() > 10000
        || arr(p, "warnings").len() > 50000
        || arr(p, "policies").len() > 4096
        || arr(p, "translations").len() > 4096
    {
        return fail("invalid path model version or budget");
    }
    let evidence: HashSet<String> = arr(c, "evidence")
        .iter()
        .map(|e| s(e, "id").to_owned())
        .collect();
    if !refs_ok(p, "evidence_refs", &evidence, false) {
        return fail("missing path coverage evidence");
    }
    let (mut interfaces, mut folded, mut iprefix) = (
        HashSet::new(),
        HashSet::new(),
        HashMap::<String, (Ipv4Addr, u8)>::new(),
    );
    for i in arr(p, "interfaces") {
        let name = s(i, "name");
        let zone = s(i, "zone");
        if zone.len() > 64 || zone.chars().any(|ch| matches!(ch, '\r' | '\n' | '\0')) {
            return fail("invalid interface zone");
        }
        if name.is_empty()
            || name.len() > 48
            || name.chars().any(|ch| matches!(ch, '\r' | '\n' | '\0'))
            || !folded.insert(name.to_lowercase())
            || s(i, "physical").is_empty()
            || !refs_ok(i, "evidence_refs", &evidence, true)
        {
            return fail("invalid path interface identity or evidence");
        }
        interfaces.insert(name.to_owned());
        if let Some(level) = i.get("security_level").and_then(Value::as_i64) {
            if !(0..=100).contains(&level) {
                return fail("invalid interface security level");
            }
        }
        let address = s(i, "address");
        if !address.is_empty() {
            let Some(prefix) = ip4_prefix(address) else {
                return fail("invalid interface IPv4 prefix");
            };
            iprefix.insert(name.to_owned(), prefix);
        } else if i.get("resolved").and_then(Value::as_bool).unwrap_or(false) {
            return fail("resolved interface has no address");
        }
    }
    let mut routes = HashSet::new();
    for r in arr(p, "routes") {
        let id = s(r, "id");
        let prefix = s(r, "prefix");
        if id.is_empty()
            || id.len() > 256
            || !routes.insert(id.to_owned())
            || !p4canonical(prefix)
            || !interfaces.contains(s(r, "interface"))
            || !refs_ok(r, "evidence_refs", &evidence, true)
        {
            return fail("invalid path route or evidence");
        }
        match s(r, "kind") {
            "connected" => {
                if r.get("distance").and_then(Value::as_i64).unwrap_or(0) != 0
                    || !s(r, "next_hop").is_empty()
                {
                    return fail("invalid connected route");
                }
                let Some((ip, bits)) = iprefix.get(s(r, "interface")) else {
                    return fail("invalid connected route");
                };
                let (rp, rb) = ip4_prefix(prefix).unwrap();
                let mask = if *bits == 0 {
                    0
                } else {
                    u32::MAX << (32 - *bits)
                };
                if u32::from(*ip) & mask != u32::from(rp) || bits != &rb {
                    return fail("invalid connected route");
                }
            }
            "static" => {
                if !p4addr(s(r, "next_hop")) {
                    return fail("invalid static route");
                }
                let d = r.get("distance").and_then(Value::as_i64).unwrap_or(0);
                if !(1..=254).contains(&d) {
                    return fail("invalid static route");
                }
            }
            _ => return fail("unknown route kind"),
        }
    }
    let mut natids = HashSet::new();
    for r in arr(p, "nat_rules") {
        let id = s(r, "id");
        let real = s(r, "real_prefix");
        let mapped = s(r, "mapped_prefix");
        if id.is_empty()
            || id.len() > 256
            || !natids.insert(id.to_owned())
            || s(r, "object_name").is_empty()
            || !interfaces.contains(s(r, "real_interface"))
            || !interfaces.contains(s(r, "mapped_interface"))
            || !refs_ok(r, "evidence_refs", &evidence, true)
            || !p4canonical(real)
            || !p4canonical(mapped)
            || ip4_prefix(real).unwrap().1 != ip4_prefix(mapped).unwrap().1
        {
            return fail("invalid static NAT mapping or evidence");
        }
        let proto = s(r, "protocol");
        let rp = r.get("real_port").and_then(Value::as_i64).unwrap_or(0);
        let mp = r.get("mapped_port").and_then(Value::as_i64).unwrap_or(0);
        if proto.is_empty() {
            if rp != 0 || mp != 0 {
                return fail("NAT port mapping has no protocol");
            }
        } else if !matches!(proto, "tcp" | "udp")
            || !(1..=65535).contains(&rp)
            || !(1..=65535).contains(&mp)
        {
            return fail("invalid NAT port mapping");
        }
    }
    let source_lines = arr(c, "source_lines");
    for w in arr(p, "warnings") {
        let line = w.get("line").and_then(Value::as_i64).unwrap_or(0);
        if s(w, "code").is_empty()
            || line < 0
            || line as usize > source_lines.len()
            || s(w, "message").len() > 256
        {
            return fail("invalid path warning");
        }
    }
    if !vendor_path {
        if !arr(p, "policies").is_empty()
            || !arr(p, "translations").is_empty()
            || p.get("policy_complete")
                .and_then(Value::as_bool)
                .unwrap_or(false)
        {
            return fail("ASA model cannot claim foreign policy semantics");
        }
        for i in arr(p, "interfaces") {
            if !s(i, "zone").is_empty() {
                return fail("ASA model cannot claim zones");
            }
        }
        return Ok(json!({"valid":true}));
    }
    if !arr(p, "nat_rules").is_empty()
        || p.get("inter_interface")
            .and_then(Value::as_bool)
            .unwrap_or(false)
    {
        return fail("vendor model cannot claim ASA NAT semantics");
    }
    let mut policies = HashSet::new();
    let mut translations = HashMap::<String, Value>::new();
    for r in arr(p, "policies") {
        let id = s(r, "id");
        if id.is_empty()
            || id.len() > 256
            || !policies.insert(id.to_owned())
            || id != s(r.get("match").unwrap_or(&Value::Null), "id")
            || s(r, "from").is_empty()
            || s(r, "to").is_empty()
            || s(r, "from").len() > 64
            || s(r, "to").len() > 64
            || !refs_ok(r, "evidence_refs", &evidence, true)
            || !refs_ok(
                r.get("match").unwrap_or(&Value::Null),
                "evidence_refs",
                &evidence,
                true,
            )
        {
            return fail("invalid vendor policy identity or evidence");
        }
        vendor_match(r.get("match").unwrap_or(&Value::Null))?;
    }
    for r in arr(p, "translations") {
        let id = s(r, "id");
        if id.is_empty()
            || id.len() > 256
            || translations.contains_key(id)
            || s(r, "from").is_empty()
            || s(r, "to").is_empty()
            || s(r, "from").len() > 64
            || s(r, "to").len() > 64
            || !refs_ok(r, "evidence_refs", &evidence, true)
            || !refs_ok(
                r.get("match").unwrap_or(&Value::Null),
                "evidence_refs",
                &evidence,
                true,
            )
        {
            return fail("invalid vendor translation identity or evidence");
        }
        translations.insert(id.to_owned(), r.clone());
        vendor_match(r.get("match").unwrap_or(&Value::Null))?;
        let kind = s(r, "kind");
        if kind == "none" {
            if !s(r, "original_prefix").is_empty() || !s(r, "translated_prefix").is_empty() {
                return fail("invalid identity NAT");
            }
        } else {
            let a = s(r, "original_prefix");
            let b = s(r, "translated_prefix");
            if !matches!(kind, "snat" | "dnat")
                || !p4canonical(a)
                || !p4canonical(b)
                || ip4_prefix(a).unwrap().1 != ip4_prefix(b).unwrap().1
            {
                return fail("invalid vendor static translation");
            }
        }
        let pid = s(r, "policy_id");
        if !pid.is_empty() && !policies.contains(pid) {
            return fail("translation references missing policy");
        }
    }
    for r in arr(p, "policies") {
        let vip = s(r, "vip");
        if !vip.is_empty() && translations.get(vip).map(|x| s(x, "kind")) != Some("dnat") {
            return fail("policy references missing or incompatible VIP");
        }
        let sn = s(r, "source_nat");
        if !sn.is_empty() {
            let Some(t) = translations.get(sn) else {
                return fail("policy references missing or incompatible source NAT");
            };
            if s(t, "kind") != "snat"
                || s(t, "policy_id") != s(r, "id")
                || s(t, "from") != s(r, "from")
                || s(t, "to") != s(r, "to")
            {
                return fail("policy references missing or incompatible source NAT");
            }
        }
    }
    Ok(json!({"valid":true}))
}
