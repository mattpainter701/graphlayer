use super::*;
use std::collections::{BTreeMap, HashSet};
use std::net::{IpAddr, Ipv4Addr, Ipv6Addr};

#[derive(Clone, Default, Serialize, Deserialize)]
#[serde(rename_all = "PascalCase")]
pub struct ParsedConfig {
    pub hostname: String,
    pub context: String,
    pub lines: Vec<RawLine>,
    #[serde(rename = "ACLs")]
    pub acls: BTreeMap<String, Vec<Ace>>,
    pub bindings: Vec<Binding>,
    pub objects: BTreeMap<String, AddressObject>,
    pub service_objects: BTreeMap<String, ServiceObject>,
    pub service_groups: BTreeMap<String, ServiceGroup>,
    pub warnings: Vec<ConfigWarning>,
    #[serde(rename = "ArtifactSHA256")]
    pub artifact_sha256: String,
}

pub fn parse_running_config(input: &str, artifact_sha256: &str) -> Result<ParsedConfig, String> {
    if input.len() > 2 << 20 {
        return Err("ASA config exceeds 2097152 bytes".into());
    }
    let text = input.replace("\r\n", "\n").replace('\r', "\n");
    let lines: Vec<&str> = text.split('\n').collect();
    if lines.len() > 50_000 {
        return Err("ASA config exceeds 50000 lines".into());
    }
    let mut c = ParsedConfig {
        context: "default".into(),
        artifact_sha256: artifact_sha256.into(),
        ..Default::default()
    };
    for (i, line) in lines.iter().enumerate() {
        if line.len() > 8192 {
            return Err(format!("ASA config line {} exceeds 8192 bytes", i + 1));
        }
        c.lines.push(RawLine {
            number: (i + 1) as i32,
            text: (*line).into(),
        });
    }
    let mut contexts = 0;
    let mut multiple = false;
    for line in c.lines.clone() {
        let f: Vec<&str> = line.text.split_whitespace().collect();
        if f.is_empty() {
            continue;
        }
        match f[0] {
            "hostname" if f.len() >= 2 => c.hostname = f[1].into(),
            "context" if f.len() >= 2 => {
                c.context = f[1].into();
                contexts += 1;
            }
            "mode" if f.get(1) == Some(&"multiple") => multiple = true,
            "access-group" => {
                if f.len() < 2 {
                    c.warnings.push(warning(
                        "unsupported_binding",
                        "Malformed access-group directive",
                        line.number,
                        "",
                    ));
                    continue;
                }
                let supported = f.len() == 5 && matches!(f[2], "in" | "out") && f[3] == "interface";
                let interface = if f.len() > 4 && f[3] == "interface" {
                    f[4]
                } else {
                    ""
                };
                let direction = if f.len() > 2 {
                    f[2].to_ascii_lowercase()
                } else {
                    "unsupported".into()
                };
                c.bindings.push(Binding {
                    acl_name: f[1].into(),
                    interface: interface.into(),
                    direction,
                    line: line.number,
                    supported,
                });
                if !supported {
                    c.warnings.push(warning(
                        "unsupported_binding",
                        "Unsupported access-group binding retained",
                        line.number,
                        f[1],
                    ));
                }
            }
            "object" | "object-group" if f.len() >= 3 && f[1] == "network" => {
                let group = f[0] == "object-group";
                let o = c.objects.entry(f[2].into()).or_default();
                if !o.name.is_empty() && o.group != group {
                    return Err(format!(
                        "ASA object name {:?} is declared as both a network object and group",
                        f[2]
                    ));
                }
                o.name = f[2].into();
                o.group = group;
                o.lines.push(line.number);
            }
            _ => {}
        }
    }
    services::parse_definitions(&mut c);
    let mut names: BTreeMap<String, &str> = BTreeMap::new();
    for (name, o) in &c.objects {
        names.insert(
            name.clone(),
            if o.group {
                "network_group"
            } else {
                "network_object"
            },
        );
    }
    for (items, kind) in [
        (
            &c.service_objects.keys().cloned().collect::<Vec<_>>(),
            "service_object",
        ),
        (
            &c.service_groups.keys().cloned().collect::<Vec<_>>(),
            "service_group",
        ),
    ] {
        for name in items {
            if let Some(prior) = names.insert(name.clone(), kind) {
                if prior != kind {
                    return Err(format!(
                        "ASA object name {name:?} is reused by {prior} and {kind}"
                    ));
                }
            }
        }
    }
    let mut current = String::new();
    for line in &c.lines {
        let f: Vec<&str> = line.text.split_whitespace().collect();
        if f.is_empty() {
            continue;
        }
        if f.len() >= 3 && matches!(f[0], "object" | "object-group") && f[1] == "network" {
            current = f[2].into();
            continue;
        }
        if !line.text.starts_with([' ', '\t']) {
            current.clear();
            continue;
        }
        let Some(o) = c.objects.get_mut(&current) else {
            continue;
        };
        let member = match f.as_slice() {
            ["host", address, ..] => host(address).unwrap_or_default(),
            ["subnet", address, mask, ..] => mask_prefix(address, mask).unwrap_or_default(),
            ["network-object", "host", address, ..] => host(address).unwrap_or_default(),
            ["network-object", "object", name, ..] => format!("@network_object:{name}"),
            ["network-object", "object-group", name, ..] => format!("@network_group:{name}"),
            ["network-object", address, mask, ..] => mask_prefix(address, mask).unwrap_or_default(),
            ["group-object", name, ..] => format!("@network_group:{name}"),
            _ => String::new(),
        };
        if !member.is_empty() {
            o.members.push(member);
            o.member_lines.push(line.number);
            o.lines.push(line.number);
        } else if !(harmless(&f) || (!o.group && f[0].eq_ignore_ascii_case("nat"))) {
            o.unsupported_lines.push(line.number);
            o.lines.push(line.number);
        }
    }
    for line in &c.lines {
        let f: Vec<&str> = line.text.split_whitespace().collect();
        if f.len() < 2 || f[0] != "access-list" || f.get(2) == Some(&"remark") {
            continue;
        }
        let mut ace = match parse_ace(line, &c) {
            Some(a) => a,
            None => {
                c.warnings.push(warning(
                    "unsupported_ace",
                    "ACE retained as unresolved",
                    line.number,
                    f[1],
                ));
                unsupported_ace(f[1], line)
            }
        };
        let prior = c.acls.entry(f[1].into()).or_default();
        if !ace.explicit_order {
            ace.order = prior.iter().map(|a| a.order).max().unwrap_or(0).max(0) + 1;
        }
        prior.push(ace);
    }
    for (name, aces) in &c.acls {
        let mut positions = HashSet::new();
        let mut explicit = 0;
        let mut implicit = 0;
        for a in aces {
            if a.explicit_order {
                explicit += 1;
                if !positions.insert(a.order) {
                    c.warnings.push(warning(
                        "ambiguous_acl_order",
                        "Duplicate explicit ACE sequence numbers; order is ambiguous",
                        a.line,
                        name,
                    ));
                }
            } else {
                implicit += 1;
            }
        }
        if explicit > 0 {
            let message = if implicit > 0 {
                "ACL mixes explicit sequence numbers and unnumbered ACEs; order cannot be established safely"
            } else {
                "Explicit line sequence directives are not replayed; order cannot be established safely"
            };
            c.warnings
                .push(warning("ambiguous_acl_order", message, aces[0].line, name));
        }
    }
    if multiple || contexts > 1 {
        c.warnings.push(warning("ambiguous_context","Multiple ASA security contexts appear in the export; ACL chains cannot be attributed safely",0,""));
    }
    if c.hostname.is_empty() {
        c.hostname = "unnamed-asa".into();
        c.warnings.push(warning(
            "missing_hostname",
            "No hostname was configured; using a local snapshot label",
            0,
            "",
        ));
    }
    Ok(c)
}

fn unsupported_ace(acl: &str, line: &RawLine) -> Ace {
    Ace {
        acl_name: acl.into(),
        line: line.number,
        raw: line.text.clone(),
        action: "deny".into(),
        protocol: "ip".into(),
        source: "any".into(),
        destination: "any".into(),
        unsupported: vec!["vendor_default".into()],
        source_lines: vec![line.number],
        ..Default::default()
    }
}
fn parse_ace(line: &RawLine, c: &ParsedConfig) -> Option<Ace> {
    let f: Vec<&str> = line.text.split_whitespace().collect();
    if f.len() < 6 || f[0] != "access-list" {
        return None;
    }
    let mut base = 2;
    let mut order = 0;
    if f[base] == "line" {
        order = f.get(base + 1)?.parse::<i32>().ok()?;
        if order < 1 {
            return None;
        }
        base += 2;
    }
    if f.len() <= base + 4 || f[base] != "extended" {
        return None;
    }
    let mut a = Ace {
        acl_name: f[1].into(),
        line: line.number,
        raw: line.text.clone(),
        order,
        explicit_order: base > 2,
        action: f[base + 1].into(),
        protocol: f[base + 2].to_ascii_lowercase(),
        enabled: true,
        source_lines: vec![line.number],
        ..Default::default()
    };
    if !matches!(a.action.as_str(), "permit" | "deny") {
        return None;
    }
    if matches!(a.protocol.as_str(), "object-group" | "object") {
        if a.protocol == "object-group" {
            a.service_group = f.get(base + 3)?.to_string();
        } else {
            a.service_object = f.get(base + 3)?.to_string();
        }
    } else if !matches!(a.protocol.as_str(), "tcp" | "udp" | "icmp" | "ip") {
        return None;
    }
    if f.iter().any(|s| s.eq_ignore_ascii_case("not")) {
        return None;
    }
    a.disabled = f.contains(&"inactive");
    let mut i = base + 3 + usize::from(!a.service_group.is_empty() || !a.service_object.is_empty());
    (a.source, i) = address_expr(&f, i)?;
    if i < f.len() {
        let (destination, ambiguous) = destination_group(&f, i, c);
        if ambiguous {
            return None;
        }
        if !destination {
            (a.source_port, i) = port_expr(&f, i, &a.protocol)?;
        }
    }
    (a.destination, i) = address_expr(&f, i)?;
    if i < f.len() {
        (a.destination_port, i) = port_expr(&f, i, &a.protocol)?;
    }
    while i < f.len() {
        match f[i] {
            "inactive" => a.disabled = true,
            "time-range" => {
                a.unsupported.push("schedule".into());
                i += 1;
            }
            "log" | "log-input" => {}
            "established" | "fragments" | "object-group-search" => {
                a.unsupported.push("stateful".into())
            }
            _ => return None,
        }
        i += 1;
    }
    Some(a)
}
fn destination_group(f: &[&str], i: usize, c: &ParsedConfig) -> (bool, bool) {
    if i + 1 >= f.len() || f[i] != "object-group" {
        return (false, false);
    }
    let network = c.objects.contains_key(f[i + 1]);
    let service = c.service_groups.contains_key(f[i + 1]);
    if network && service {
        return (false, true);
    }
    if network {
        return (true, false);
    }
    if service {
        return (false, false);
    }
    if let Some((_, next)) = address_expr(f, i) {
        if next == f.len()
            || f.get(next).is_some_and(|s| {
                matches!(
                    *s,
                    "inactive"
                        | "log"
                        | "log-input"
                        | "time-range"
                        | "established"
                        | "fragments"
                        | "object-group-search"
                        | "eq"
                        | "range"
                        | "lt"
                        | "gt"
                        | "neq"
                )
            })
        {
            return (true, false);
        }
    }
    (false, false)
}
pub(super) fn address_expr(f: &[&str], i: usize) -> Option<(String, usize)> {
    match *f.get(i)? {
        "any" | "any4" | "any6" => Some((f[i].into(), i + 1)),
        "host" => Some((f.get(i + 1)?.to_string(), i + 2)),
        "object" | "object-group" => Some((
            format!(
                "@{}:{}",
                if f[i] == "object" {
                    "network_object"
                } else {
                    "network_group"
                },
                f.get(i + 1)?
            ),
            i + 2,
        )),
        value => {
            if let Some(prefix) = normalized_prefix(value) {
                return Some((prefix, i + 1));
            }
            if i + 1 < f.len() {
                if let Some(prefix) = mask_prefix(value, f[i + 1]) {
                    return Some((prefix, i + 2));
                }
                if value.parse::<Ipv4Addr>().is_ok() && f[i + 1].parse::<Ipv4Addr>().is_ok() {
                    return None;
                }
            }
            Some((
                if value.parse::<IpAddr>().is_ok() {
                    value.into()
                } else {
                    format!("@{value}")
                },
                i + 1,
            ))
        }
    }
}
pub(super) fn port_expr(f: &[&str], i: usize, p: &str) -> Option<(String, usize)> {
    if i >= f.len() {
        return Some((String::new(), i));
    }
    match f[i] {
        "eq" => Some((services::service_port_for(p, f.get(i + 1)?)?, i + 2)),
        "range" => Some((
            format!(
                "{}-{}",
                services::service_port_for(p, f.get(i + 1)?)?,
                services::service_port_for(p, f.get(i + 2)?)?
            ),
            i + 3,
        )),
        "lt" | "gt" | "neq" => {
            let (ports, _) = services::port_alternatives(&f[i..], p)?;
            Some((ports.join("|"), i + 2))
        }
        "object-group" => Some((format!("@{}", f.get(i + 1)?), i + 2)),
        _ => Some((String::new(), i)),
    }
}
fn host(value: &str) -> Option<String> {
    let ip = value.parse::<IpAddr>().ok()?;
    if matches!(ip,IpAddr::V6(v)if v.to_ipv4_mapped().is_some()) {
        return None;
    }
    Some(ip.to_string())
}
pub(super) fn mask_prefix(address: &str, mask: &str) -> Option<String> {
    let ip = address.parse::<Ipv4Addr>().ok()?;
    let bits = crate::mask_bits(mask)?;
    Some(format!("{}/{}", crate::mask_ip(ip, bits), bits))
}
fn normalized_prefix(value: &str) -> Option<String> {
    let (ip, bits) = value.split_once('/')?;
    let bits = bits.parse::<u32>().ok()?;
    match ip.parse::<IpAddr>().ok()? {
        IpAddr::V4(ip) if bits <= 32 => Some(format!(
            "{}/{}",
            Ipv4Addr::from(
                u32::from(ip)
                    & if bits == 0 {
                        0
                    } else {
                        u32::MAX << (32 - bits)
                    }
            ),
            bits
        )),
        IpAddr::V6(ip) if bits <= 128 => Some(format!(
            "{}/{}",
            Ipv6Addr::from(
                u128::from(ip)
                    & if bits == 0 {
                        0
                    } else {
                        u128::MAX << (128 - bits)
                    }
            ),
            bits
        )),
        _ => None,
    }
}

pub fn expand_address(c: &ParsedConfig, expr: &str) -> (Vec<String>, Vec<i32>, Option<String>) {
    if !expr.starts_with('@') {
        return (vec![expr.into()], vec![], None);
    }
    let (name, kind) = network_reference(&expr[1..]);
    let mut out = vec![];
    let mut lines = vec![];
    fn visit(
        c: &ParsedConfig,
        name: &str,
        kind: &str,
        depth: usize,
        stack: &mut HashSet<String>,
        out: &mut Vec<String>,
        lines: &mut Vec<i32>,
    ) -> Result<(), String> {
        if depth > 16 {
            return Err("network object nesting limit".into());
        }
        if stack.contains(name) {
            return Err("cyclic network object".into());
        }
        let o = c.objects.get(name).ok_or("unresolved network object")?;
        if depth == 0 && !kind.is_empty() && (kind == "network_group") != o.group {
            return Err("network object kind mismatch".into());
        }
        lines.extend(&o.lines);
        if !o.unsupported_lines.is_empty() {
            return Err("network object contains unsupported members".into());
        }
        if o.members.is_empty() {
            return Err("unresolved network object".into());
        }
        stack.insert(name.into());
        for member in &o.members {
            if let Some(raw) = member.strip_prefix('@') {
                let (refname, expected) = network_reference(raw);
                visit(c, refname, kind, depth + 1, stack, out, lines)?;
                if !expected.is_empty()
                    && c.objects
                        .get(refname)
                        .is_some_and(|o| (expected == "network_group") != o.group)
                {
                    return Err("network object kind mismatch".into());
                }
            } else {
                out.push(member.clone());
                if out.len() > 256 {
                    return Err("ACE expansion limit".into());
                }
            }
        }
        stack.remove(name);
        Ok(())
    }
    let err = visit(c, name, kind, 0, &mut HashSet::new(), &mut out, &mut lines).err();
    if err.is_some() {
        (vec![], lines, err)
    } else {
        (out, unique(&lines), None)
    }
}
