use super::*;
use serde_json::{json, Value};
use std::collections::{HashMap, HashSet};

struct Projection<'a> {
    parsed: &'a ParsedConfig,
    source: Vec<Value>,
    evidence: Vec<Value>,
    chains: Vec<Value>,
    origins: Vec<Value>,
    objects: Vec<Value>,
    refs: HashMap<i32, String>,
}
fn warn(code: &str, message: &str, line: i32) -> Value {
    let mut v = json!({"code":code,"message":message});
    if line != 0 {
        v["line"] = json!(line);
    }
    v
}
fn optional(value: &mut Value, key: &str, text: &str) {
    if !text.is_empty() {
        value[key] = json!(text);
    }
}
fn optional_list(value: &mut Value, key: &str, list: &[String]) {
    if !list.is_empty() {
        value[key] = json!(list);
    }
}
impl Projection<'_> {
    fn mark(&mut self, lines: &[i32]) {
        for n in lines {
            if *n > 0 && (*n as usize) <= self.parsed.lines.len() {
                self.source[*n as usize - 1]["text"] = json!(crate::sanitize_line(
                    &self.parsed.lines[*n as usize - 1].text
                ));
            }
        }
    }
    fn add_evidence(&mut self, lines: &[i32], kind: &str, summary: &str) -> String {
        self.mark(lines);
        let id = crate::evidence_id(&self.parsed.artifact_sha256, lines);
        self.evidence.push(json!({"id":id,"start_line":lines.iter().min().copied().unwrap_or(0),"end_line":lines.iter().max().copied().unwrap_or(0),"kind":kind,"summary":summary}));
        id
    }
    fn inventory_ref(&mut self, line: i32, kind: &str, summary: &str) -> String {
        if let Some(id) = self.refs.get(&line) {
            return id.clone();
        }
        self.mark(&[line]);
        let id = crate::evidence_id(&self.parsed.artifact_sha256, &[line]);
        if !self.evidence.iter().any(|v| v["id"] == id) {
            self.add_evidence(&[line], kind, summary);
        }
        self.refs.insert(line, id.clone());
        id
    }
    fn inventory(&mut self) {
        for (name, o) in &self.parsed.objects {
            let kind = if o.group {
                "network_group"
            } else {
                "network_object"
            };
            let mut refs = vec![];
            for line in unique(&o.lines) {
                refs.push(self.inventory_ref(
                    line,
                    "object_member",
                    "Network object declaration or member",
                ));
            }
            let mut members = vec![];
            let mut resolved = !o.members.is_empty() && o.unsupported_lines.is_empty();
            for (i, m) in o.members.iter().enumerate() {
                let mut member = json!({"kind":"address","value":m.trim_start_matches('@')});
                if let Some(line) = o.member_lines.get(i).filter(|n| **n > 0) {
                    member["evidence_refs"] = json!([self.inventory_ref(
                        *line,
                        "object_member",
                        "Network object member"
                    )]);
                }
                if let Some(raw) = m.strip_prefix('@') {
                    let (refname, _) = network_reference(raw);
                    member["kind"] = json!("reference");
                    member.as_object_mut().unwrap().remove("value");
                    member["object_id"] = json!(object_id(&self.parsed.objects, m));
                    if !self.parsed.objects.contains_key(refname) {
                        resolved = false;
                    }
                }
                members.push(member);
            }
            if expand_address(self.parsed, &format!("@{name}")).2.is_some() {
                resolved = false;
            }
            for line in &o.unsupported_lines {
                members.push(json!({"kind":"unsupported","value":"unsupported network member","evidence_refs":[self.inventory_ref(*line,"object_member","Unsupported network object member")]}));
            }
            let mut object = json!({"id":format!("{kind}:{name}"),"name":name,"kind":kind,"resolved":resolved,"members":members,"evidence_refs":refs});
            if let Some(line) = o.unsupported_lines.first() {
                object["warnings"] = json!([warn(
                    "unsupported_object_member",
                    "Network object has unsupported members",
                    *line
                )]);
            }
            self.objects.push(object);
        }
        for (name, o) in &self.parsed.service_objects {
            let mut refs = vec![];
            for line in unique(&o.lines) {
                refs.push(self.inventory_ref(
                    line,
                    "service_object",
                    "Service object declaration or member",
                ));
            }
            let resolved = !o.protocol.is_empty() && o.unsupported_lines.is_empty();
            let mut members = vec![];
            if !o.protocol.is_empty() {
                for sp in services::port_values(&o.source_port) {
                    for dp in services::port_values(&o.destination_port) {
                        let mut member = json!({"kind":"service","protocol":o.protocol});
                        optional(&mut member, "source_port", &sp);
                        optional(&mut member, "destination_port", &dp);
                        if o.member_line > 0 {
                            member["evidence_refs"] = json!([self.inventory_ref(
                                o.member_line,
                                "service_member",
                                "Service object definition"
                            )]);
                        }
                        members.push(member);
                    }
                }
            }
            for line in &o.unsupported_lines {
                members.push(json!({"kind":"unsupported","value":"unsupported service object member","evidence_refs":[self.inventory_ref(*line,"service_member","Unsupported service object member")]}));
            }
            let mut object = json!({"id":format!("service_object:{name}"),"name":name,"kind":"service_object","resolved":resolved,"members":members,"evidence_refs":refs});
            optional(&mut object, "protocol", &o.protocol);
            if !resolved {
                object["warnings"] = json!([warn(
                    "unresolved_service_object",
                    "Service object has unsupported or missing service definition",
                    0
                )]);
            }
            self.objects.push(object);
        }
        for (name, g) in &self.parsed.service_groups {
            let mut refs = vec![];
            for line in unique(&g.lines) {
                refs.push(self.inventory_ref(
                    line,
                    "service_group",
                    "Service group declaration or member",
                ));
            }
            let mut resolved = !g.members.is_empty() && g.unsupported_lines.is_empty();
            let mut members = vec![];
            for m in &g.members {
                let mut member = json!({"kind":m.kind,"evidence_refs":[self.inventory_ref(m.line,"service_member","Service group member")]});
                optional(&mut member, "protocol", &m.protocol);
                optional(&mut member, "source_port", &m.source_port);
                optional(&mut member, "destination_port", &m.destination_port);
                if m.kind == "reference" {
                    member["object_id"] = json!(format!("{}:{}", m.reference_kind, m.reference));
                    let exists = match m.reference_kind.as_str() {
                        "service_object" => self.parsed.service_objects.contains_key(&m.reference),
                        "service_group" => self.parsed.service_groups.contains_key(&m.reference),
                        _ => false,
                    };
                    resolved &= exists;
                }
                members.push(member);
            }
            if services::expand_group(self.parsed, name).2.is_some() {
                resolved = false;
            }
            let mut object = json!({"id":format!("service_group:{name}"),"name":name,"kind":"service_group","resolved":resolved,"members":members,"evidence_refs":refs});
            optional(&mut object, "protocol", &g.protocol);
            if let Some(line) = g.unsupported_lines.first() {
                object["warnings"] = json!([warn(
                    "unsupported_service_member",
                    "Service group has unsupported members",
                    *line
                )]);
            }
            self.objects.push(object);
        }
    }
}

pub fn parse_asa_with_hash(input: &str, hash: &str) -> Result<Value, String> {
    let parsed = parse_running_config(input, hash)?;
    let mut p = Projection {
        parsed: &parsed,
        source: parsed
            .lines
            .iter()
            .map(|l| json!({"number":l.number,"text":"[OMITTED NON-POLICY CONFIGURATION]"}))
            .collect(),
        evidence: vec![],
        chains: vec![],
        origins: vec![],
        objects: vec![],
        refs: HashMap::new(),
    };
    let mut warnings: Vec<Value> = parsed
        .warnings
        .iter()
        .map(|w| warn(&w.code, &w.message, w.line))
        .collect();
    let ambiguous_context = parsed
        .warnings
        .iter()
        .any(|w| w.code == "ambiguous_context");
    let mut seen = HashSet::new();
    let mut seen_binding = HashSet::new();
    for b in &parsed.bindings {
        let key = format!(
            "{}/{}/{}/{}",
            b.acl_name.to_ascii_lowercase(),
            b.interface.to_ascii_lowercase(),
            b.direction.to_ascii_lowercase(),
            b.supported
        );
        if !seen_binding.insert(key) {
            continue;
        }
        let Some(aces) = parsed.acls.get(&b.acl_name).filter(|v| !v.is_empty()) else {
            warnings.push(warn("empty_acl", "Bound ACL has no parsed ACEs", b.line));
            continue;
        };
        let mut aces = aces.clone();
        aces.sort_by_key(|a| a.order);
        let mut id = format!("asa:{}:{}:{}", b.acl_name, b.interface, b.direction);
        if !b.supported {
            id.push_str(&format!(":unsupported:{}", b.line));
        }
        let mut complete = true;
        let mut chain_warnings = vec![];
        let mut ambiguous_binding = !b.supported;
        if !b.supported {
            complete = false;
            chain_warnings.push(warn(
                "unsupported_binding",
                "Attachment syntax is retained but cannot be evaluated safely",
                b.line,
            ));
        }
        if b.supported
            && parsed.bindings.iter().any(|other| {
                other.supported
                    && other.interface.eq_ignore_ascii_case(&b.interface)
                    && other.direction.eq_ignore_ascii_case(&b.direction)
                    && other.acl_name != b.acl_name
            })
        {
            ambiguous_binding = true;
            complete = false;
            chain_warnings.push(warn(
                "ambiguous_binding",
                "Multiple ACLs are bound to the same interface and direction",
                b.line,
            ));
        }
        if ambiguous_context {
            complete = false;
            chain_warnings.push(warn(
                "ambiguous_context",
                "Firewall context is ambiguous",
                b.line,
            ));
        }
        let mut chain_refs = vec![p.add_evidence(&[b.line], "binding", "ACL attachment")];
        let mut rules = vec![];
        for mut a in aces {
            for w in &parsed.warnings {
                if w.code == "unsupported_ace" && w.line == a.line {
                    complete = false;
                    chain_warnings.push(warn(
                        "unsupported_ace",
                        "ACE syntax could not be normalized and may affect any flow",
                        a.line,
                    ));
                }
                if w.code == "ambiguous_acl_order" && w.acl_name == b.acl_name {
                    complete = false;
                }
            }
            let (mut sources, src_lines, src_error) = expand_address(&parsed, &a.source);
            let (mut destinations, dst_lines, dst_error) = expand_address(&parsed, &a.destination);
            let (mut terms, service_lines, service_error) = services::rule_terms(&parsed, &a);
            let mut lines = vec![a.line];
            lines.extend(src_lines);
            lines.extend(dst_lines);
            lines.extend(service_lines);
            lines.push(b.line);
            let lines = unique(&lines)
                .into_iter()
                .filter(|n| *n > 0)
                .collect::<Vec<_>>();
            let eref = p.add_evidence(
                &lines,
                "access_rule",
                &format!("{} {} ACE {}", b.acl_name, a.action, a.order),
            );
            chain_refs.push(eref.clone());
            if src_error.is_some() {
                sources = vec![a.source.clone()];
                a.unsupported.push("object_expansion".into());
                chain_warnings.push(warn(
                    "unresolved_object",
                    "Source network object could not be safely resolved",
                    a.line,
                ));
            }
            if dst_error.is_some() {
                destinations = vec![a.destination.clone()];
                a.unsupported.push("object_expansion".into());
                chain_warnings.push(warn(
                    "unresolved_object",
                    "Destination network object could not be safely resolved",
                    a.line,
                ));
            }
            let unresolved = || services::Term {
                protocol: "unresolved-service-protocol".into(),
                source: String::new(),
                destination: String::new(),
            };
            if service_error.is_some() {
                terms = vec![unresolved()];
                a.unsupported.push("object_expansion".into());
                chain_warnings.push(warn(
                    "unresolved_service",
                    "Service object could not be safely resolved",
                    a.line,
                ));
            }
            if sources.len() * destinations.len() * terms.len() > 256 {
                sources = vec![a.source.clone()];
                destinations = vec![a.destination.clone()];
                terms = vec![unresolved()];
                a.unsupported.push("object_expansion".into());
            }
            for source in &sources {
                for destination in &destinations {
                    for term in &terms {
                        let mut unsupported = a.unsupported.clone();
                        for w in &parsed.warnings {
                            if w.code == "ambiguous_acl_order" && w.acl_name == b.acl_name {
                                unsupported.push("vendor_default".into());
                            }
                        }
                        if ambiguous_binding {
                            unsupported.push("vendor_default".into());
                        }
                        let unsupported: Vec<String> = unique(&unsupported)
                            .into_iter()
                            .filter(|s| !s.is_empty())
                            .collect();
                        let position = rules.len();
                        let rid = format!("{id}:{}:{}", a.line, position + 1);
                        let protocol = if term.protocol == "ip" {
                            "any"
                        } else {
                            &term.protocol
                        };
                        let mut rule = json!({"id":rid,"position":position,"enabled":!a.disabled,"action":if a.action.eq_ignore_ascii_case("permit"){"permit"}else{"deny"},"source":source,"destination":destination,"protocol":protocol,"source_port":term.source,"destination_port":term.destination,"evidence_refs":[eref]});
                        optional_list(&mut rule, "unsupported", &unsupported);
                        rules.push(rule);
                        let mut origin = json!({"rule_id":rid,"acl_name":b.acl_name,"line":a.line});
                        if a.source.starts_with('@') {
                            origin["source_object_ids"] =
                                json!([object_id(&parsed.objects, &a.source)]);
                        }
                        if a.destination.starts_with('@') {
                            origin["destination_object_ids"] =
                                json!([object_id(&parsed.objects, &a.destination)]);
                        }
                        let mut services = vec![];
                        if !a.service_group.is_empty() {
                            services.push(format!("service_group:{}", a.service_group));
                        }
                        if !a.service_object.is_empty() {
                            services.push(format!("service_object:{}", a.service_object));
                        }
                        for value in [&a.source_port, &a.destination_port] {
                            if let Some(name) = value.strip_prefix('@') {
                                services.push(format!("service_group:{name}"));
                            }
                        }
                        services.sort();
                        services.dedup();
                        optional_list(&mut origin, "service_object_ids", &services);
                        p.origins.push(origin);
                        if rules.len() > 10_000 {
                            return Err("ASA configuration has too many expanded ACL rules".into());
                        }
                    }
                }
            }
        }
        seen.insert(b.acl_name.clone());
        if b.direction.eq_ignore_ascii_case("out") {
            complete = false;
            chain_warnings.push(warn(
                "unsupported_direction",
                "Outbound ACL direction is retained but not evaluatable in this milestone",
                b.line,
            ));
        }
        p.chains.push(json!({"id":id,"name":b.acl_name,"interface":b.interface,"direction":b.direction,"complete":complete,"rules":rules,"evidence_refs":chain_refs,"warnings":chain_warnings}));
    }
    for (name, aces) in &parsed.acls {
        if !seen.contains(name) {
            warnings.push(warn(
                "unattached_acl",
                "ACL is not attached with a supported access-group binding",
                aces.first().map_or(0, |a| a.line),
            ));
        }
    }
    p.inventory();
    let path = crate::parse_asa_path(crate::Input {
        artifact_sha256: hash.into(),
        lines: parsed
            .lines
            .iter()
            .map(|l| crate::ConfigLine {
                number: l.number,
                text: l.text.clone(),
            })
            .collect(),
        network_objects: parsed
            .objects
            .iter()
            .map(|(name, o)| {
                (
                    name.clone(),
                    crate::NetworkObject {
                        group: o.group,
                        members: o.members.clone(),
                        unsupported_lines: o.unsupported_lines.clone(),
                    },
                )
            })
            .collect(),
    });
    for evidence in path.evidence {
        if !p.evidence.iter().any(|v| v["id"] == evidence.id) {
            p.evidence
                .push(serde_json::to_value(evidence).map_err(|_| "internal_error")?);
        }
    }
    for update in path.source_line_updates {
        if update.number > 0 && (update.number as usize) <= p.source.len() {
            p.source[update.number as usize - 1]["text"] = json!(update.text);
        }
    }
    let mut result = json!({"vendor":"cisco-asa","parser_version":"asa-config-v3","hostname":parsed.hostname,"context":parsed.context,"artifact_sha256":hash,"source_lines":p.source,"evidence":p.evidence,"chains":p.chains,"warnings":warnings,"capabilities":["inbound attached extended ACL evaluation","outbound ACLs retained as partial chains","IPv4/IPv6 literals and CIDR","TCP/UDP/ICMP/IP wildcard","network and service objects and nested groups","first-match order","inactive ACEs"],"path":path.path});
    if !p.objects.is_empty() {
        result["objects"] = json!(p.objects);
    }
    if !p.origins.is_empty() {
        result["rule_origins"] = json!(p.origins);
    }
    Ok(result)
}
