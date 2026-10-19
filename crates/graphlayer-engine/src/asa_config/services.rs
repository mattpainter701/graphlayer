use super::*;
use std::collections::HashSet;

pub(super) fn service_port_for(protocol: &str, name: &str) -> Option<String> {
    if let Ok(n) = name.parse::<i32>() {
        return (n > 0 && n <= 65535).then(|| n.to_string());
    }
    let n = name.to_ascii_lowercase();
    let port = match n.as_str() {
        "http" | "www" => 80,
        "https" => 443,
        "ssh" => 22,
        "domain" => 53,
        "dns" => 195,
        "smtp" => 25,
        "ftp" => 21,
        "ftp-data" => 20,
        "telnet" => 23,
        "snmp" => 161,
        "snmptrap" => 162,
        "ntp" => 123,
        "tftp" => 69,
        "ldap" => 389,
        "ldaps" => 636,
        "imap4" => 143,
        "pop3" => 110,
        "bgp" => 179,
        "syslog" => 514,
        "sqlnet" => 1521,
        "tacacs" => 49,
        "radius" => 1645,
        "radius-acct" => 1646,
        "sip" => 5060,
        "kerberos" => 750,
        _ => return None,
    };
    let tcp = matches!(
        n.as_str(),
        "https"
            | "ssh"
            | "smtp"
            | "ftp"
            | "ftp-data"
            | "telnet"
            | "ldap"
            | "ldaps"
            | "imap4"
            | "pop3"
            | "bgp"
            | "sqlnet"
    );
    let udp = matches!(
        n.as_str(),
        "snmp" | "snmptrap" | "tftp" | "radius" | "radius-acct" | "ntp" | "dns" | "syslog"
    );
    let dual = matches!(
        n.as_str(),
        "http" | "www" | "domain" | "kerberos" | "sip" | "tacacs"
    );
    if (protocol == "tcp-udp" && !dual)
        || (protocol == "tcp" && (udp || !(tcp || dual)))
        || (protocol == "udp" && (tcp || !(udp || dual)))
    {
        return None;
    }
    Some(port.to_string())
}
pub(super) fn port_alternatives(f: &[&str], protocol: &str) -> Option<(Vec<String>, usize)> {
    let op = f.first()?.to_ascii_lowercase();
    let p = service_port_for(protocol, f.get(1)?)?;
    match op.as_str() {
        "eq" => Some((vec![p], 2)),
        "range" => {
            let b = service_port_for(protocol, f.get(2)?)?;
            if p.parse::<i32>().ok()? > b.parse::<i32>().ok()? {
                return None;
            }
            Some((vec![format!("{p}-{b}")], 3))
        }
        "lt" | "gt" | "neq" => {
            let n = p.parse::<i32>().ok()?;
            let mut out = vec![];
            if matches!(op.as_str(), "lt" | "neq") && n > 1 {
                out.push(format!("1-{}", n - 1));
            }
            if matches!(op.as_str(), "gt" | "neq") && n < 65535 {
                out.push(format!("{}-65535", n + 1));
            }
            (!out.is_empty()).then_some((out, 2))
        }
        _ => None,
    }
}
fn spec(f: &[&str], allow_dual: bool) -> Option<(String, String, String)> {
    let p = f.first()?.to_ascii_lowercase();
    if !matches!(p.as_str(), "tcp" | "udp") && !(p == "tcp-udp" && allow_dual) {
        return None;
    }
    let (mut source, mut destination) = (String::new(), String::new());
    let mut i = 1;
    while i < f.len() {
        let which = f[i].to_ascii_lowercase();
        if !matches!(which.as_str(), "source" | "destination") {
            return None;
        }
        i += 1;
        let (v, n) = port_alternatives(&f[i..], &p)?;
        i += n;
        let value = v.join("|");
        let slot = if which == "source" {
            &mut source
        } else {
            &mut destination
        };
        if !slot.is_empty() {
            return None;
        }
        *slot = value;
    }
    Some((p, source, destination))
}
pub(super) fn parse_definitions(c: &mut ParsedConfig) {
    let (mut kind, mut name) = (String::new(), String::new());
    for line in &c.lines {
        let f: Vec<&str> = line.text.split_whitespace().collect();
        if f.is_empty() {
            continue;
        }
        if f.len() >= 3 && f[0] == "object" && f[1] == "service" {
            kind = "object".into();
            name = f[2].into();
            let o = c.service_objects.entry(name.clone()).or_default();
            o.name = name.clone();
            o.lines.push(line.number);
            continue;
        }
        if f.len() >= 3 && f[0] == "object-group" && f[1] == "service" {
            kind = "group".into();
            name = f[2].into();
            let o = c.service_groups.entry(name.clone()).or_default();
            o.name = name.clone();
            o.protocol = f.get(3).map_or(String::new(), |p| p.to_ascii_lowercase());
            o.lines.push(line.number);
            continue;
        }
        if !line.text.starts_with([' ', '\t']) {
            kind.clear();
            name.clear();
            continue;
        }
        if name.is_empty() {
            continue;
        }
        if kind == "object" {
            let o = c.service_objects.get_mut(&name).unwrap();
            o.lines.push(line.number);
            if f.len() >= 2 && f[0] == "service" {
                if let Some((p, sp, dp)) = spec(&f[1..], false) {
                    o.protocol = p;
                    o.source_port = sp;
                    o.destination_port = dp;
                    o.member_line = line.number;
                } else {
                    o.unsupported_lines.push(line.number);
                }
            } else if !harmless(&f) {
                o.unsupported_lines.push(line.number);
            }
        } else if kind == "group" {
            let g = c.service_groups.get_mut(&name).unwrap();
            g.lines.push(line.number);
            let mut m = ServiceMember {
                line: line.number,
                ..Default::default()
            };
            let mut extra = vec![];
            match f.as_slice() {
                ["group-object", reference, ..] => {
                    m.kind = "reference".into();
                    m.reference = (*reference).into();
                    m.reference_kind = "service_group".into();
                }
                ["service-object", tail @ ..] if !tail.is_empty() => {
                    if !g.protocol.is_empty() {
                        m.kind = "unsupported".into();
                    } else if tail[0].eq_ignore_ascii_case("object") && tail.len() >= 2 {
                        m.kind = "reference".into();
                        m.reference = tail[1].into();
                        m.reference_kind = "service_object".into();
                    } else if let Some((p, sp, dp)) = spec(tail, true) {
                        m.kind = "service".into();
                        m.protocol = p;
                        m.source_port = sp;
                        m.destination_port = dp;
                    } else {
                        m.kind = "unsupported".into();
                    }
                }
                ["port-object", op, ..] if f.len() >= 3 => {
                    let ports = if matches!(op.to_ascii_lowercase().as_str(), "eq" | "range") {
                        port_alternatives(&f[1..], &g.protocol)
                    } else {
                        None
                    };
                    if let Some((ports, _)) =
                        ports.filter(|_| matches!(g.protocol.as_str(), "tcp" | "udp" | "tcp-udp"))
                    {
                        m.kind = "service".into();
                        m.protocol = g.protocol.clone();
                        m.destination_port = ports[0].clone();
                        for port in &ports[1..] {
                            extra.push(ServiceMember {
                                kind: "service".into(),
                                protocol: g.protocol.clone(),
                                destination_port: port.clone(),
                                line: line.number,
                                ..Default::default()
                            });
                        }
                    } else {
                        m.kind = "unsupported".into();
                    }
                }
                _ => {
                    if !harmless(&f) {
                        m.kind = "unsupported".into();
                    }
                }
            }
            if m.kind == "unsupported" {
                g.unsupported_lines.push(line.number);
            }
            if !m.kind.is_empty() {
                g.members.push(m);
            }
            g.members.extend(extra);
        }
    }
}
#[derive(Clone)]
pub(super) struct Term {
    pub protocol: String,
    pub source: String,
    pub destination: String,
}
pub(super) fn port_values(value: &str) -> Vec<String> {
    if value.is_empty() {
        vec![String::new()]
    } else {
        value.split('|').map(String::from).collect()
    }
}
fn append_terms(terms: &mut Vec<Term>, p: &str, source: &str, destination: &str) {
    let protocols = if p == "tcp-udp" {
        vec!["tcp", "udp"]
    } else {
        vec![p]
    };
    for s in port_values(source) {
        for d in port_values(destination) {
            for p in &protocols {
                terms.push(Term {
                    protocol: (*p).into(),
                    source: s.clone(),
                    destination: d.clone(),
                });
            }
        }
    }
}
pub(super) fn expand_group(c: &ParsedConfig, name: &str) -> (Vec<Term>, Vec<i32>, Option<String>) {
    fn visit(
        c: &ParsedConfig,
        name: &str,
        stack: &mut HashSet<String>,
        depth: usize,
        parent: &str,
        terms: &mut Vec<Term>,
        lines: &mut Vec<i32>,
    ) -> Result<(), String> {
        if depth > 16 {
            return Err("service nesting limit".into());
        }
        if stack.contains(name) {
            return Err("cyclic service group".into());
        }
        let g = c.service_groups.get(name).ok_or("missing service group")?;
        if !parent.is_empty() && !g.protocol.is_empty() && g.protocol != parent {
            return Err("incompatible nested service group protocol".into());
        }
        lines.extend(&g.lines);
        if !g.unsupported_lines.is_empty() {
            return Err("unsupported service member".into());
        }
        if g.members.is_empty() {
            return Err("empty service group".into());
        }
        stack.insert(name.into());
        for m in &g.members {
            lines.push(m.line);
            match m.kind.as_str() {
                "unsupported" => return Err("unsupported service member".into()),
                "reference" => match m.reference_kind.as_str() {
                    "service_object" => {
                        let o = c
                            .service_objects
                            .get(&m.reference)
                            .ok_or("missing service object")?;
                        lines.extend(&o.lines);
                        if !o.unsupported_lines.is_empty() || o.protocol.is_empty() {
                            return Err("unresolved service object".into());
                        }
                        append_terms(terms, &o.protocol, &o.source_port, &o.destination_port);
                    }
                    "service_group" => {
                        if !c.service_groups.contains_key(&m.reference) {
                            return Err("missing service reference".into());
                        }
                        visit(c, &m.reference, stack, depth + 1, &g.protocol, terms, lines)?;
                    }
                    _ => return Err("unknown service reference kind".into()),
                },
                "service" => append_terms(terms, &m.protocol, &m.source_port, &m.destination_port),
                _ => {}
            }
            if terms.len() > 256 {
                return Err("service expansion limit".into());
            }
        }
        stack.remove(name);
        Ok(())
    }
    let (mut terms, mut lines) = (vec![], vec![]);
    let err = visit(c, name, &mut HashSet::new(), 0, "", &mut terms, &mut lines).err();
    if err.is_some() {
        terms.clear();
    }
    (terms, unique(&lines), err)
}
pub(super) fn rule_terms(c: &ParsedConfig, a: &Ace) -> (Vec<Term>, Vec<i32>, Option<String>) {
    if !a.service_group.is_empty() {
        if !a.source_port.is_empty() || !a.destination_port.is_empty() {
            return (
                vec![],
                vec![],
                Some("service group ACE has additional port predicates".into()),
            );
        }
        return expand_group(c, &a.service_group);
    }
    if !a.service_object.is_empty() {
        if !a.source_port.is_empty() || !a.destination_port.is_empty() {
            return (
                vec![],
                vec![],
                Some("service object ACE has additional port predicates".into()),
            );
        }
        let Some(o) = c.service_objects.get(&a.service_object) else {
            return (vec![], vec![], Some("unresolved service object".into()));
        };
        if o.protocol.is_empty() || !o.unsupported_lines.is_empty() {
            return (
                vec![],
                o.lines.clone(),
                Some("unresolved service object".into()),
            );
        }
        let mut terms = vec![];
        append_terms(&mut terms, &o.protocol, &o.source_port, &o.destination_port);
        return (terms, unique(&o.lines), None);
    }
    let (mut source, mut destination) = (
        port_values(&a.source_port),
        port_values(&a.destination_port),
    );
    let mut lines = vec![];
    for (value, is_source) in [(&a.source_port, true), (&a.destination_port, false)] {
        let Some(name) = value.strip_prefix('@') else {
            continue;
        };
        let (terms, refs, err) = expand_group(c, name);
        lines.extend(refs);
        if let Some(err) = err {
            return (vec![], unique(&lines), Some(err));
        }
        let error = |message: &str| (vec![], unique(&lines), Some(message.into()));
        if terms.is_empty() {
            return error("empty port group");
        }
        let Some(group) = c.service_groups.get(name) else {
            return error("port group protocol mismatch");
        };
        if group.protocol != a.protocol
            && !(group.protocol == "tcp-udp" && matches!(a.protocol.as_str(), "tcp" | "udp"))
        {
            return error("port group protocol mismatch");
        }
        let mut ports = vec![];
        for term in terms {
            if !term.source.is_empty() || term.destination.is_empty() {
                return error("service group is not a port-only group");
            }
            if term.protocol != a.protocol {
                if group.protocol == "tcp-udp" && matches!(term.protocol.as_str(), "tcp" | "udp") {
                    continue;
                }
                return error("port group protocol mismatch");
            }
            ports.push(term.destination);
        }
        let ports = unique(&ports);
        if ports.is_empty() {
            return error("port group has no alternatives for ACE protocol");
        }
        if is_source {
            source = ports;
        } else {
            destination = ports;
        }
        if source.len() * destination.len() > 256 {
            return error("port group cross-product limit");
        }
    }
    if source.len() * destination.len() > 256 {
        return (
            vec![],
            unique(&lines),
            Some("port predicate cross-product limit".into()),
        );
    }
    let mut terms = vec![];
    for s in source {
        for d in &destination {
            terms.push(Term {
                protocol: a.protocol.clone(),
                source: s.clone(),
                destination: d.clone(),
            });
        }
    }
    (terms, unique(&lines), None)
}
