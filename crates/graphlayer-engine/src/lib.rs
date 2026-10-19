#![forbid(unsafe_code)]

pub mod vendor_parsers;
pub mod investigation_engine;
pub mod connectors;
pub mod asa_config;
pub mod dispatch;

use serde::{Deserialize, Serialize};
use sha2::{Digest, Sha256};
use std::collections::{BTreeMap, HashMap, HashSet};
use std::net::Ipv4Addr;

pub const ASA_PATH_VERSION: &str = "asa-path-v1";
pub const MAX_PATH_INTERFACES: usize = 1024;
pub const MAX_PATH_ROUTES: usize = 4096;
pub const MAX_PATH_NAT_RULES: usize = 4096;

#[derive(Clone, Debug, Deserialize, Serialize)]
#[serde(deny_unknown_fields)]
pub struct Input {
    pub artifact_sha256: String,
    pub lines: Vec<ConfigLine>,
    #[serde(default)]
    pub network_objects: BTreeMap<String, NetworkObject>,
}
#[derive(Clone, Debug, Deserialize, Serialize)]
#[serde(deny_unknown_fields)]
pub struct ConfigLine {
    pub number: i32,
    pub text: String,
}
#[derive(Clone, Debug, Default, Deserialize, Serialize)]
#[serde(deny_unknown_fields)]
pub struct NetworkObject {
    #[serde(default)]
    pub group: bool,
    #[serde(default)]
    pub members: Vec<String>,
    #[serde(default)]
    pub unsupported_lines: Vec<i32>,
}
#[derive(Clone, Debug, Serialize)]
pub struct Output {
    pub path: PathConfig,
    pub evidence: Vec<Evidence>,
    pub source_line_updates: Vec<SourceLine>,
}
#[derive(Clone, Debug, Serialize)]
pub struct PathConfig {
    pub version: String,
    pub interfaces: Vec<PathInterface>,
    pub routes: Vec<PathRoute>,
    pub nat_rules: Vec<PathNatRule>,
    pub routing_complete: bool,
    pub nat_complete: bool,
    pub inter_interface: bool,
    pub warnings: Vec<Warning>,
    pub evidence_refs: Vec<String>,
}
#[derive(Clone, Debug, Serialize)]
pub struct PathInterface {
    pub name: String,
    pub physical: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub address: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub security_level: Option<i32>,
    pub shutdown: bool,
    pub resolved: bool,
    pub evidence_refs: Vec<String>,
}
#[derive(Clone, Debug, Serialize)]
pub struct PathRoute {
    pub id: String,
    pub kind: String,
    pub prefix: String,
    pub interface: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub next_hop: Option<String>,
    pub distance: i32,
    pub evidence_refs: Vec<String>,
}
#[derive(Clone, Debug, Serialize)]
pub struct PathNatRule {
    pub id: String,
    pub object_name: String,
    pub real_interface: String,
    pub mapped_interface: String,
    pub real_prefix: String,
    pub mapped_prefix: String,
    pub evidence_refs: Vec<String>,
}
#[derive(Clone, Debug, Serialize)]
pub struct Warning {
    pub code: String,
    pub message: String,
    #[serde(skip_serializing_if = "is_zero")]
    pub line: i32,
}
#[derive(Clone, Debug, Serialize)]
pub struct Evidence {
    pub id: String,
    pub start_line: i32,
    pub end_line: i32,
    pub kind: String,
    pub summary: String,
}
#[derive(Clone, Debug, Serialize)]
pub struct SourceLine {
    pub number: i32,
    pub text: String,
}
fn is_zero(value: &i32) -> bool {
    *value == 0
}

#[derive(Default)]
struct Builder {
    evidence: Vec<Evidence>,
    evidence_ids: HashSet<String>,
    refs: Vec<String>,
    source: BTreeMap<i32, String>,
    warnings: Vec<Warning>,
    routing_complete: bool,
    nat_complete: bool,
}
impl Builder {
    fn evidence(
        &mut self,
        hash: &str,
        lines: &[i32],
        kind: &str,
        summary: &str,
        input: &Input,
    ) -> Option<String> {
        let line = *lines.first()?;
        if !valid_line(input, line) {
            return None;
        }
        let id = evidence_id(hash, lines);
        for n in lines {
            if let Some(l) = source_line(input, *n) {
                self.source.insert(*n, sanitize_line(&l.text));
            }
        }
        if self.evidence_ids.insert(id.clone()) {
            self.evidence.push(Evidence {
                id: id.clone(),
                start_line: line,
                end_line: line,
                kind: kind.into(),
                summary: summary.into(),
            });
        }
        Some(id)
    }
    fn refs(
        &mut self,
        hash: &str,
        lines: &[i32],
        kind: &str,
        summary: &str,
        input: &Input,
    ) -> Vec<String> {
        let mut out = Vec::new();
        for n in unique_ints(lines) {
            if let Some(r) = self.evidence(hash, &[n], kind, summary, input) {
                out.push(r);
            }
        }
        self.refs.extend(out.clone());
        out
    }
    fn warn(
        &mut self,
        hash: &str,
        code: &str,
        message: &str,
        line: i32,
        routing: bool,
        input: &Input,
    ) {
        self.warnings.push(Warning {
            code: code.into(),
            message: message.into(),
            line,
        });
        if routing {
            self.routing_complete = false
        } else {
            self.nat_complete = false
        }
        if let Some(r) = self.evidence(hash, &[line], "unsupported_path_input", message, input) {
            self.refs.push(r);
            if source_line(input, line).is_some() {
                self.source
                    .insert(line, "[OMITTED UNSUPPORTED PATH CONFIGURATION]".into());
            }
        }
    }
}

pub fn parse_asa_path(input: Input) -> Output {
    let mut b = Builder {
        routing_complete: true,
        nat_complete: true,
        ..Default::default()
    };
    let mut path = PathConfig {
        version: ASA_PATH_VERSION.into(),
        interfaces: vec![],
        routes: vec![],
        nat_rules: vec![],
        routing_complete: true,
        nat_complete: true,
        inter_interface: false,
        warnings: vec![],
        evidence_refs: vec![],
    };
    let (mut ints, int_lines, unsupported_int) = parse_interfaces(&input.lines);
    if ints.len() > MAX_PATH_INTERFACES {
        b.warn(
            &input.artifact_sha256,
            "path_interface_limit",
            "Interface count exceeds the supported path input limit",
            0,
            true,
            &input,
        );
        ints.truncate(MAX_PATH_INTERFACES);
    }
    let mut lookup: HashMap<String, usize> = HashMap::new();
    let mut dup = vec![false; ints.len()];
    for i in 0..ints.len() {
        let key = ints[i].name.to_lowercase();
        if ints[i].name.is_empty() || ints[i].name.len() > 48 {
            ints[i].resolved = false
        } else if let Some(prev) = lookup.get(&key).copied() {
            ints[i].resolved = false;
            ints[prev].resolved = false;
            dup[i] = true
        } else {
            lookup.insert(key, i);
        }
    }
    for i in 0..ints.len() {
        ints[i].evidence_refs = b.refs(
            &input.artifact_sha256,
            &int_lines[i],
            "path_interface",
            "Interface forwarding properties",
            &input,
        );
        if ints[i].name.is_empty() || ints[i].name.len() > 48 {
            let line = int_lines[i].first().copied().unwrap_or(0);
            b.warn(
                &input.artifact_sha256,
                "unresolved_interface_name",
                "Interface name is missing or exceeds the supported length",
                line,
                true,
                &input,
            );
            continue;
        }
        if dup[i] {
            b.warn(
                &input.artifact_sha256,
                "ambiguous_interface_name",
                "Interface name is assigned to multiple interface stanzas",
                int_lines[i].first().copied().unwrap_or(0),
                true,
                &input,
            );
            continue;
        }
        path.interfaces.push(ints[i].clone());
    }
    for n in unsupported_int {
        b.warn(
            &input.artifact_sha256,
            "unsupported_interface",
            "Interface forwarding properties are incomplete",
            n,
            true,
            &input,
        )
    }
    let mut byname: HashMap<String, PathInterface> = HashMap::new();
    for it in &ints {
        byname.insert(it.name.to_lowercase(), it.clone());
    }
    for it in &path.interfaces {
        if it.resolved {
            if let Some(a) = &it.address {
                if let Some((ip, bits)) = parse_prefix(a) {
                    let refid = it.evidence_refs.first().cloned().unwrap_or_default();
                    path.routes.push(PathRoute {
                        id: format!("asa-connected:{}:{}", refid, it.name.to_lowercase()),
                        kind: "connected".into(),
                        prefix: format!("{}/{}", mask_ip(ip, bits), bits),
                        interface: it.name.clone(),
                        next_hop: None,
                        distance: 0,
                        evidence_refs: it.evidence_refs.clone(),
                    });
                }
            }
        }
    }
    let mut seen = HashSet::new();
    let mut route_idx = HashMap::<String, usize>::new();
    for l in &input.lines {
        let f = fields(&l.text);
        if f.is_empty() {
            continue;
        }
        let cmd = f[0].to_lowercase();
        match cmd.as_str() {
            "route" => {
                if let Some(mut r) = parse_route(&f, l.number, &input.artifact_sha256) {
                    if r.interface.eq_ignore_ascii_case("null0") {
                        b.warn(
                            &input.artifact_sha256,
                            "unsupported_route",
                            "Null interface routes are outside the supported path model",
                            l.number,
                            true,
                            &input,
                        );
                        continue;
                    }
                    if let Some(it) = byname
                        .get(&r.interface.to_lowercase())
                        .filter(|x| x.resolved)
                    {
                        r.interface = it.name.clone()
                    } else {
                        b.warn(
                            &input.artifact_sha256,
                            "unresolved_route_interface",
                            "Static route interface cannot be resolved to one configured interface",
                            l.number,
                            true,
                            &input,
                        );
                        continue;
                    }
                    r.evidence_refs = b.refs(
                        &input.artifact_sha256,
                        &[l.number],
                        "static_route",
                        "Static IPv4 route",
                        &input,
                    );
                    let k = format!(
                        "{}/{}/{}/{}",
                        r.prefix,
                        r.interface.to_lowercase(),
                        r.next_hop.as_deref().unwrap_or(""),
                        r.distance
                    );
                    if seen.insert(k.clone()) {
                        route_idx.insert(k, path.routes.len());
                        path.routes.push(r)
                    } else if let Some(i) = route_idx.get(&k).copied() {
                        path.routes[i].evidence_refs.extend(r.evidence_refs);
                        path.routes[i].evidence_refs = unique_strings(&path.routes[i].evidence_refs)
                    }
                } else {
                    b.warn(
                        &input.artifact_sha256,
                        "unsupported_route",
                        "Static route syntax or options are outside the supported path model",
                        l.number,
                        true,
                        &input,
                    )
                }
            }
            "ipv6" if f.len() > 1 && f[1].eq_ignore_ascii_case("route") => b.warn(
                &input.artifact_sha256,
                "unsupported_route",
                "IPv6 route is outside the IPv4 path model",
                l.number,
                true,
                &input,
            ),
            "router" if f.len() > 1 && is_dynamic(&f[1]) => b.warn(
                &input.artifact_sha256,
                "dynamic_routing",
                "Dynamic routing configuration is outside the static path model",
                l.number,
                true,
                &input,
            ),
            "policy-route" | "pbr" => b.warn(
                &input.artifact_sha256,
                "policy_routing",
                "Policy routing configuration is outside the static path model",
                l.number,
                true,
                &input,
            ),
            "route-map" => b.warn(
                &input.artifact_sha256,
                "policy_routing",
                "Route-map routing behavior is outside the static path model",
                l.number,
                true,
                &input,
            ),
            "firewall" if f.len() > 1 && f[1].eq_ignore_ascii_case("transparent") => b.warn(
                &input.artifact_sha256,
                "transparent_mode",
                "Transparent firewall mode is outside the routed path model",
                l.number,
                true,
                &input,
            ),
            "crypto"
                if f.len() > 1
                    && (f[1].eq_ignore_ascii_case("map")
                        || f[1].eq_ignore_ascii_case("dynamic-map")) =>
            {
                b.warn(
                    &input.artifact_sha256,
                    "vpn_routing",
                    "VPN crypto-map routing is outside the static path model",
                    l.number,
                    true,
                    &input,
                )
            }
            "tunnel-group" => b.warn(
                &input.artifact_sha256,
                "vpn_routing",
                "VPN tunnel configuration is outside the static path model",
                l.number,
                true,
                &input,
            ),
            "nat-control" => b.warn(
                &input.artifact_sha256,
                "unsupported_nat",
                "Global NAT control is outside the supported path model",
                l.number,
                false,
                &input,
            ),
            "static" | "global" => b.warn(
                &input.artifact_sha256,
                "unsupported_nat",
                "Legacy NAT configuration is outside the supported object NAT model",
                l.number,
                false,
                &input,
            ),
            "ip" if f.len() > 2
                && f[1].eq_ignore_ascii_case("verify")
                && f[2].eq_ignore_ascii_case("reverse-path") =>
            {
                b.warn(
                    &input.artifact_sha256,
                    "reverse_path_verification",
                    "Reverse-path verification is outside the static path model",
                    l.number,
                    true,
                    &input,
                )
            }
            "ip" if f.len() > 1 && f[1].eq_ignore_ascii_case("policy") => b.warn(
                &input.artifact_sha256,
                "policy_routing",
                "IP policy routing is outside the static path model",
                l.number,
                true,
                &input,
            ),
            "mode" if f.len() > 1 && f[1].eq_ignore_ascii_case("multiple") => b.warn(
                &input.artifact_sha256,
                "ambiguous_context",
                "Multiple-context routing attribution is incomplete",
                l.number,
                true,
                &input,
            ),
            "mode" if f.len() > 1 && f[1].eq_ignore_ascii_case("transparent") => b.warn(
                &input.artifact_sha256,
                "transparent_mode",
                "Transparent firewall mode is outside the routed path model",
                l.number,
                true,
                &input,
            ),
            "context" => b.warn(
                &input.artifact_sha256,
                "ambiguous_context",
                "Multiple-context routing attribution is incomplete",
                l.number,
                true,
                &input,
            ),
            "vrf" => b.warn(
                &input.artifact_sha256,
                "unsupported_vrf",
                "VRF routing is outside the supported path model",
                l.number,
                true,
                &input,
            ),
            _ => {}
        }
    }
    if path.routes.len() > MAX_PATH_ROUTES {
        path.routes.truncate(MAX_PATH_ROUTES);
        b.warn(
            &input.artifact_sha256,
            "path_route_limit",
            "Route count exceeds the supported path input limit",
            0,
            true,
            &input,
        )
    }
    for l in &input.lines {
        let f = fields(&l.text);
        if !f.is_empty()
            && !l.text.starts_with(' ')
            && !l.text.starts_with('\t')
            && f[0].eq_ignore_ascii_case("nat")
        {
            b.warn(
                &input.artifact_sha256,
                "unsupported_nat",
                "Manual or twice NAT is outside the supported object NAT path model",
                l.number,
                false,
                &input,
            )
        }
    }
    let (nats, unsupported_nat) = parse_nat(&input);
    for mut n in nats {
        if let Some(i) = byname
            .get(&n.rule.real_interface.to_lowercase())
            .filter(|x| x.resolved)
        {
            n.rule.real_interface = i.name.clone()
        } else {
            b.warn(
                &input.artifact_sha256,
                "unresolved_nat_interface",
                "Static NAT real interface cannot be resolved",
                n.nat_line,
                false,
                &input,
            );
            continue;
        }
        if let Some(i) = byname
            .get(&n.rule.mapped_interface.to_lowercase())
            .filter(|x| x.resolved)
        {
            n.rule.mapped_interface = i.name.clone()
        } else {
            b.warn(
                &input.artifact_sha256,
                "unresolved_nat_interface",
                "Static NAT mapped interface cannot be resolved",
                n.nat_line,
                false,
                &input,
            );
            continue;
        }
        n.rule.evidence_refs = b.refs(
            &input.artifact_sha256,
            &[n.declaration, n.address_line, n.nat_line],
            "static_object_nat",
            "Static object NAT mapping",
            &input,
        );
        path.nat_rules.push(n.rule);
    }
    for n in unsupported_nat {
        b.warn(
            &input.artifact_sha256,
            "unsupported_nat",
            "Object NAT syntax or address mapping is outside the supported path model",
            n,
            false,
            &input,
        )
    }
    if path.nat_rules.len() > MAX_PATH_NAT_RULES {
        path.nat_rules.truncate(MAX_PATH_NAT_RULES);
        b.warn(
            &input.artifact_sha256,
            "path_nat_limit",
            "NAT rule count exceeds the supported path input limit",
            0,
            false,
            &input,
        )
    }
    for l in &input.lines {
        let f = fields(&l.text);
        if f.len() == 3
            && f[0].eq_ignore_ascii_case("same-security-traffic")
            && f[1].eq_ignore_ascii_case("permit")
            && f[2].eq_ignore_ascii_case("inter-interface")
        {
            path.inter_interface = true;
            if let Some(r) = b.evidence(
                &input.artifact_sha256,
                &[l.number],
                "path_setting",
                "Inter-interface traffic setting",
                &input,
            ) {
                b.refs.push(r)
            }
        } else if f.len() == 4
            && f[0].eq_ignore_ascii_case("no")
            && f[1].eq_ignore_ascii_case("same-security-traffic")
            && f[2].eq_ignore_ascii_case("permit")
            && f[3].eq_ignore_ascii_case("inter-interface")
        {
            path.inter_interface = false;
            if let Some(r) = b.evidence(
                &input.artifact_sha256,
                &[l.number],
                "path_setting",
                "Inter-interface traffic setting",
                &input,
            ) {
                b.refs.push(r)
            }
        } else if (f.len() == 3
            && f[0].eq_ignore_ascii_case("same-security-traffic")
            && f[1].eq_ignore_ascii_case("permit")
            && f[2].eq_ignore_ascii_case("intra-interface"))
            || (f.len() == 4
                && f[0].eq_ignore_ascii_case("no")
                && f[1].eq_ignore_ascii_case("same-security-traffic")
                && f[2].eq_ignore_ascii_case("permit")
                && f[3].eq_ignore_ascii_case("intra-interface"))
        {
            b.warn(
                &input.artifact_sha256,
                "intra_interface_traffic",
                "Intra-interface forwarding is outside the supported path model",
                l.number,
                true,
                &input,
            )
        }
    }
    path.evidence_refs = unique_strings(&b.refs);
    path.interfaces
        .sort_by(|a, c| a.name.cmp(&c.name).then(a.physical.cmp(&c.physical)));
    path.routes.sort_by(|a, c| a.id.cmp(&c.id));
    path.nat_rules.sort_by(|a, c| a.id.cmp(&c.id));
    path.warnings = b.warnings;
    path.routing_complete = b.routing_complete;
    path.nat_complete = b.nat_complete;
    let updates = b
        .source
        .into_iter()
        .map(|(number, text)| SourceLine { number, text })
        .collect();
    Output {
        path,
        evidence: b.evidence,
        source_line_updates: updates,
    }
}

fn source_line(i: &Input, n: i32) -> Option<&ConfigLine> {
    let idx = usize::try_from(n.checked_sub(1)?).ok()?;
    i.lines.get(idx).filter(|l| l.number == n)
}
fn valid_line(i: &Input, n: i32) -> bool {
    source_line(i, n).is_some()
}
fn fields(s: &str) -> Vec<String> {
    s.split_whitespace().map(str::to_string).collect()
}
fn unique_ints(a: &[i32]) -> Vec<i32> {
    let mut h = HashSet::new();
    a.iter()
        .copied()
        .filter(|x| *x > 0 && h.insert(*x))
        .collect()
}
fn unique_strings(a: &[String]) -> Vec<String> {
    let mut h = HashSet::new();
    a.iter()
        .filter(|s| !s.is_empty() && h.insert(s.as_str().to_owned()))
        .cloned()
        .collect()
}
fn evidence_id(hash: &str, lines: &[i32]) -> String {
    let mut h = Sha256::new();
    h.update(hash.as_bytes());
    for n in lines {
        h.update(format!(":{}", n).as_bytes())
    }
    format!("sha256:{:x}", h.finalize())
}
fn sanitize_line(s: &str) -> String {
    let low = s.to_lowercase();
    for term in [
        "password",
        "passwd",
        "secret",
        "community",
        "pre-shared-key",
        "key-string",
        "private-key",
        "ldap-login-password",
        "radius-common-pw",
        "enable-password",
        "username",
    ] {
        if contains_word_phrase(&low, term) {
            return "[REDACTED SENSITIVE CONFIGURATION]".into();
        }
    }
    if contains_crypto_ca_authenticate(&low) {
        return "[REDACTED SENSITIVE CONFIGURATION]".into();
    }
    let up = s.to_uppercase();
    if ((up.contains("BEGIN ")) && (up.contains("PRIVATE KEY") || up.contains("CERTIFICATE")))
        || ((up.contains("END ")) && (up.contains("PRIVATE KEY") || up.contains("CERTIFICATE")))
    {
        return "[REDACTED KEY OR CERTIFICATE MATERIAL]".into();
    }
    s.into()
}
fn contains_word_phrase(s: &str, term: &str) -> bool {
    let mut from = 0;
    while let Some(i) = s[from..].find(term) {
        let a = from + i;
        let z = a + term.len();
        let left = s[..a]
            .chars()
            .next_back()
            .map(|c| !c.is_ascii_alphanumeric() && c != '_')
            .unwrap_or(true);
        let right = s[z..]
            .chars()
            .next()
            .map(|c| !c.is_ascii_alphanumeric() && c != '_')
            .unwrap_or(true);
        if left && right {
            return true;
        }
        from = z;
        if from >= s.len() {
            break;
        }
    }
    false
}
fn contains_crypto_ca_authenticate(s: &str) -> bool {
    let mut from = 0;
    while let Some(i) = s[from..].find("crypto") {
        let a = from + i;
        let mut p = a + 6;
        let left = s[..a]
            .chars()
            .next_back()
            .map(|c| !c.is_ascii_alphanumeric() && c != '_')
            .unwrap_or(true);
        if left {
            let b = p;
            while s.as_bytes().get(p).is_some_and(u8::is_ascii_whitespace) {
                p += 1
            }
            if p > b && s[p..].starts_with("ca") {
                p += 2;
                let c = p;
                while s.as_bytes().get(p).is_some_and(u8::is_ascii_whitespace) {
                    p += 1
                }
                if p > c && s[p..].starts_with("authenticate") {
                    p += 12;
                    if s[p..]
                        .chars()
                        .next()
                        .map(|c| !c.is_ascii_alphanumeric() && c != '_')
                        .unwrap_or(true)
                    {
                        return true;
                    }
                }
            }
        }
        from = a + 6;
        if from >= s.len() {
            break;
        }
    }
    false
}
fn parse_interfaces(lines: &[ConfigLine]) -> (Vec<PathInterface>, Vec<Vec<i32>>, Vec<i32>) {
    let mut out = Vec::new();
    let mut refs = Vec::new();
    let mut unsupported = Vec::new();
    let mut cur: Option<usize> = None;
    let (mut sn, mut sa, mut sl) = (false, false, false);
    for l in lines {
        let t = l.text.trim();
        let f = fields(t);
        if f.is_empty() {
            continue;
        }
        if !l.text.starts_with(' ') && !l.text.starts_with('\t') {
            cur = None;
            if f[0].eq_ignore_ascii_case("interface") && f.len() >= 2 {
                out.push(PathInterface {
                    name: String::new(),
                    physical: f[1..].join(" "),
                    address: None,
                    security_level: None,
                    shutdown: false,
                    resolved: true,
                    evidence_refs: vec![],
                });
                refs.push(vec![l.number]);
                cur = Some(out.len() - 1);
                sn = false;
                sa = false;
                sl = false
            }
            continue;
        }
        let Some(i) = cur else { continue };
        let cmd = f[0].to_lowercase();
        match cmd.as_str() {
            "nameif" => {
                refs[i].push(l.number);
                if f.len() != 2 || sn {
                    out[i].resolved = false;
                    unsupported.push(l.number)
                } else {
                    out[i].name = f[1].clone();
                    sn = true
                }
            }
            "ip" => {
                if f.len() > 1 && f[1].eq_ignore_ascii_case("address") {
                    refs[i].push(l.number);
                    if sa
                        || f.len() < 3
                        || f[2].eq_ignore_ascii_case("dhcp")
                        || f[2].eq_ignore_ascii_case("pppoe")
                    {
                        out[i].resolved = false;
                        unsupported.push(l.number)
                    } else if let Some(ip) = ipv4(&f[2]) {
                        let mask = if f.len() >= 4 && !f[3].eq_ignore_ascii_case("standby") {
                            f[3].clone()
                        } else {
                            "255.255.255.255".into()
                        };
                        if let Some(bits) = mask_bits(&mask) {
                            out[i].address = Some(format!("{}/{}", ip, bits));
                            sa = true;
                            if f.len() > 4 {
                                out[i].resolved = false;
                                unsupported.push(l.number)
                            }
                        } else {
                            out[i].resolved = false;
                            unsupported.push(l.number)
                        }
                    } else {
                        out[i].resolved = false;
                        unsupported.push(l.number)
                    }
                } else {
                    out[i].resolved = false;
                    unsupported.push(l.number)
                }
            }
            "security-level" => {
                refs[i].push(l.number);
                if f.len() != 2 || sl {
                    out[i].resolved = false;
                    unsupported.push(l.number)
                } else if let Ok(n) = f[1].parse::<i32>() {
                    if (0..=100).contains(&n) {
                        out[i].security_level = Some(n);
                        sl = true
                    } else {
                        out[i].resolved = false;
                        unsupported.push(l.number)
                    }
                } else {
                    out[i].resolved = false;
                    unsupported.push(l.number)
                }
            }
            "shutdown" => {
                refs[i].push(l.number);
                out[i].shutdown = true
            }
            "no" => {
                if f.len() > 1 && f[1].eq_ignore_ascii_case("shutdown") {
                    refs[i].push(l.number);
                    out[i].shutdown = false
                } else if f.len() < 2 || !is_metadata(&f[1].to_lowercase()) {
                    out[i].resolved = false;
                    unsupported.push(l.number)
                }
            }
            "ipv6" | "bridge-group" | "pppoe" | "management-only" | "standby" => {
                out[i].resolved = false;
                unsupported.push(l.number)
            }
            _ => {
                if !is_metadata(&cmd) {
                    out[i].resolved = false;
                    unsupported.push(l.number)
                }
            }
        }
    }
    for (i, x) in out.iter_mut().enumerate() {
        if x.name.is_empty() || x.address.is_none() {
            x.resolved = false
        }
        refs[i] = unique_ints(&refs[i])
    }
    (out, refs, unique_ints(&unsupported))
}
fn is_metadata(s: &str) -> bool {
    matches!(
        s,
        "description"
            | "speed"
            | "duplex"
            | "mtu"
            | "flowcontrol"
            | "mac-address"
            | "media-type"
            | "bandwidth"
            | "delay"
            | "monitor-interface"
    )
}
fn ipv4(s: &str) -> Option<Ipv4Addr> {
    if s.contains('%') {
        None
    } else {
        s.parse().ok()
    }
}
fn mask_bits(s: &str) -> Option<u8> {
    let x = u32::from(ipv4(s)?);
    let inv = !x;
    if inv & inv.wrapping_add(1) != 0 {
        return None;
    }
    Some(x.count_ones() as u8)
}
fn parse_prefix(s: &str) -> Option<(Ipv4Addr, u8)> {
    let (a, n) = s.split_once('/')?;
    let ip = ipv4(a)?;
    let b = n.parse::<u8>().ok()?;
    (b <= 32).then_some((ip, b))
}
fn mask_ip(ip: Ipv4Addr, bits: u8) -> Ipv4Addr {
    let x = u32::from(ip);
    let m = if bits == 0 {
        0
    } else {
        u32::MAX << (32 - bits)
    };
    Ipv4Addr::from(x & m)
}
fn is_dynamic(s: &str) -> bool {
    ["bgp", "ospf", "eigrp", "rip", "isis"]
        .iter()
        .any(|x| s.eq_ignore_ascii_case(x))
}
fn parse_route(f: &[String], line: i32, artifact: &str) -> Option<PathRoute> {
    if f.len() < 5 || !f[0].eq_ignore_ascii_case("route") {
        return None;
    }
    let dest = if f[2] == "0" { "0.0.0.0" } else { &f[2] };
    let mask = if f[3] == "0" { "0.0.0.0" } else { &f[3] };
    let ip = ipv4(dest)?;
    let bits = mask_bits(mask)?;
    let next = ipv4(&f[4])?;
    let distance = if f.len() > 5 {
        let n = f[5].parse::<i32>().ok()?;
        if !(1..=254).contains(&n) {
            return None;
        }
        n
    } else {
        1
    };
    if f.len() > 6 {
        return None;
    }
    let id = evidence_id(artifact, &[line]);
    Some(PathRoute {
        id: format!("asa-route:{}", id),
        kind: "static".into(),
        prefix: format!("{}/{}", mask_ip(ip, bits), bits),
        interface: f[1].clone(),
        next_hop: Some(next.to_string()),
        distance,
        evidence_refs: vec![],
    })
}

#[derive(Clone)]
struct PendingNat {
    rule: PathNatRule,
    declaration: i32,
    address_line: i32,
    nat_line: i32,
}
fn parse_nat(input: &Input) -> (Vec<PendingNat>, Vec<i32>) {
    #[derive(Default)]
    struct Obj {
        declarations: Vec<i32>,
        address: Option<String>,
        address_line: i32,
        nat: Vec<String>,
        nat_line: i32,
        bad: Vec<i32>,
    }
    let mut objects: HashMap<String, Obj> = HashMap::new();
    let mut uns = Vec::new();
    let mut current = String::new();
    for l in &input.lines {
        let t = l.text.trim();
        let f = fields(t);
        if f.is_empty() {
            continue;
        }
        if !l.text.starts_with(' ') && !l.text.starts_with('\t') {
            current.clear();
            if f.len() >= 3
                && f[0].eq_ignore_ascii_case("object")
                && f[1].eq_ignore_ascii_case("network")
            {
                current = f[2].clone();
                objects
                    .entry(current.clone())
                    .or_default()
                    .declarations
                    .push(l.number)
            } else if f.len() > 1
                && f[0].eq_ignore_ascii_case("object-group")
                && f[1].eq_ignore_ascii_case("network")
            {
                current.clear()
            }
            if f[0].eq_ignore_ascii_case("nat") {
                uns.push(l.number)
            }
            continue;
        }
        if current.is_empty() {
            continue;
        }
        let o = objects.get_mut(&current).unwrap();
        match f[0].to_lowercase().as_str() {
            "host" => {
                if f.len() != 2 {
                    o.bad.push(l.number);
                    continue;
                }
                if let Some(ip) = ipv4(&f[1]) {
                    let val = format!("{}/32", ip);
                    if o.address.as_ref().is_some_and(|a| a != &val) {
                        o.bad.push(l.number)
                    } else if o.address.is_none() {
                        o.address = Some(val);
                        o.address_line = l.number
                    }
                } else {
                    o.bad.push(l.number)
                }
            }
            "subnet" => {
                if f.len() != 3 {
                    o.bad.push(l.number);
                    continue;
                }
                if let (Some(ip), Some(bits)) = (ipv4(&f[1]), mask_bits(&f[2])) {
                    let val = format!("{}/{}", mask_ip(ip, bits), bits);
                    if o.address.as_ref().is_some_and(|a| a != &val) {
                        o.bad.push(l.number)
                    } else if o.address.is_none() {
                        o.address = Some(val);
                        o.address_line = l.number
                    }
                } else {
                    o.bad.push(l.number)
                }
            }
            "nat" => {
                if o.nat_line != 0 {
                    o.bad.push(l.number)
                } else {
                    o.nat = f;
                    o.nat_line = l.number
                }
            }
            "range" | "object" | "network-object" | "network-object-group" => o.bad.push(l.number),
            _ => {}
        }
    }
    let mut names: Vec<_> = objects.keys().cloned().collect();
    names.sort();
    let mut out = Vec::new();
    for name in names {
        let o = &objects[&name];
        if o.nat_line == 0 {
            continue;
        }
        let source = input.network_objects.get(&name);
        if let Some(s) = source {
            uns.extend(s.unsupported_lines.iter().copied())
        }
        if !o.bad.is_empty() || o.address.is_none() || o.declarations.is_empty() {
            uns.push(o.nat_line);
            uns.extend(o.bad.iter().copied());
            continue;
        }
        let f = &o.nat;
        if f.len() != 4
            || !f[0].eq_ignore_ascii_case("nat")
            || !f[1].starts_with('(')
            || !f[1].ends_with(')')
            || !f[2].eq_ignore_ascii_case("static")
        {
            uns.push(o.nat_line);
            continue;
        }
        let ifs: Vec<_> = f[1].trim_matches(&['(', ')'][..]).split(',').collect();
        if ifs.len() != 2
            || ifs[0].is_empty()
            || ifs[1].is_empty()
            || ifs[0].eq_ignore_ascii_case("any")
            || ifs[1].eq_ignore_ascii_case("any")
        {
            uns.push(o.nat_line);
            continue;
        }
        let Some(mapped) = ipv4(&f[3]) else {
            uns.push(o.nat_line);
            continue;
        };
        let Some((real, bits)) = o.address.as_deref().and_then(parse_prefix) else {
            uns.push(o.nat_line);
            continue;
        };
        let Some(src) = source else {
            uns.push(o.nat_line);
            continue;
        };
        if src.group
            || !src.unsupported_lines.is_empty()
            || src.members.len() != 1
            || !same_object_address(&src.members[0], real, bits)
        {
            uns.push(o.nat_line);
            continue;
        }
        if mask_ip(mapped, bits) != mapped {
            uns.push(o.nat_line);
            continue;
        }
        let id = evidence_id(&input.artifact_sha256, &[o.nat_line]);
        out.push(PendingNat {
            rule: PathNatRule {
                id: format!("asa-nat:{}", id),
                object_name: name.clone(),
                real_interface: ifs[0].into(),
                mapped_interface: ifs[1].into(),
                real_prefix: format!("{}/{}", mask_ip(real, bits), bits),
                mapped_prefix: format!("{}/{}", mapped, bits),
                evidence_refs: vec![],
            },
            declaration: o.declarations[0],
            address_line: o.address_line,
            nat_line: o.nat_line,
        });
    }
    (out, unique_ints(&uns))
}
fn same_object_address(s: &str, ip: Ipv4Addr, bits: u8) -> bool {
    if let Some((a, b)) = parse_prefix(s) {
        b == bits && mask_ip(a, b) == mask_ip(ip, bits)
    } else {
        ipv4(s).is_some_and(|a| a == ip && bits == 32)
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    #[test]
    fn route_and_interface_are_canonical_and_evidence_linked() {
        let input = Input {
            artifact_sha256: "abc".into(),
            lines: vec![
                ConfigLine {
                    number: 1,
                    text: "interface GigabitEthernet0/0".into(),
                },
                ConfigLine {
                    number: 2,
                    text: " nameif outside".into(),
                },
                ConfigLine {
                    number: 3,
                    text: " ip address 192.0.2.1 255.255.255.0".into(),
                },
                ConfigLine {
                    number: 4,
                    text: "route outside 198.51.100.7 255.255.255.0 192.0.2.2".into(),
                },
            ],
            network_objects: BTreeMap::new(),
        };
        let o = parse_asa_path(input);
        assert_eq!(o.path.interfaces.len(), 1);
        assert_eq!(o.path.routes.len(), 2);
        assert!(o.path.routes.iter().any(|r| r.prefix == "198.51.100.0/24"));
        assert_eq!(
            o.path.interfaces[0].address.as_deref(),
            Some("192.0.2.1/24")
        );
        assert!(!o.evidence.is_empty());
    }
    #[test]
    fn unsupported_secrets_are_redacted_and_evidence_is_stable() {
        let input = Input {
            artifact_sha256: "abc".into(),
            lines: vec![ConfigLine {
                number: 1,
                text: "route Null0 0.0.0.0 0.0.0.0 192.0.2.1 password hunter2".into(),
            }],
            network_objects: BTreeMap::new(),
        };
        let o = parse_asa_path(input);
        assert!(o.source_line_updates[0].text == "[OMITTED UNSUPPORTED PATH CONFIGURATION]");
        assert_eq!(o.path.warnings[0].code, "unsupported_route");
        assert_eq!(o.evidence[0].id, evidence_id("abc", &[1]));
    }
    #[test]
    fn redaction_patterns_and_ipv4_masks_match_go_rules() {
        assert_eq!(
            sanitize_line("crypto   ca authenticate secret"),
            "[REDACTED SENSITIVE CONFIGURATION]"
        );
        assert_eq!(
            sanitize_line(" certificate BEGIN PRIVATE KEY material"),
            "[REDACTED KEY OR CERTIFICATE MATERIAL]"
        );
        assert_eq!(mask_bits("0.0.0.0"), Some(0));
        assert_eq!(mask_bits("255.0.255.0"), None);
    }
    #[test]
    fn static_object_nat_requires_matching_projection_and_binds_interfaces() {
        let lines = vec![
            ConfigLine {
                number: 1,
                text: "interface GigabitEthernet0/0".into(),
            },
            ConfigLine {
                number: 2,
                text: " nameif inside".into(),
            },
            ConfigLine {
                number: 3,
                text: " ip address 10.0.0.1 255.255.255.0".into(),
            },
            ConfigLine {
                number: 4,
                text: "interface GigabitEthernet0/1".into(),
            },
            ConfigLine {
                number: 5,
                text: " nameif outside".into(),
            },
            ConfigLine {
                number: 6,
                text: " ip address 203.0.113.1 255.255.255.0".into(),
            },
            ConfigLine {
                number: 7,
                text: "object network server".into(),
            },
            ConfigLine {
                number: 8,
                text: " host 10.0.0.10".into(),
            },
            ConfigLine {
                number: 9,
                text: " nat (inside,outside) static 203.0.113.10".into(),
            },
        ];
        let input = Input {
            artifact_sha256: "abc".into(),
            lines,
            network_objects: BTreeMap::from([(
                "server".into(),
                NetworkObject {
                    group: false,
                    members: vec!["10.0.0.10".into()],
                    unsupported_lines: vec![],
                },
            )]),
        };
        let o = parse_asa_path(input);
        assert_eq!(o.path.nat_rules.len(), 1);
        let nat = &o.path.nat_rules[0];
        assert_eq!(nat.real_prefix, "10.0.0.10/32");
        assert_eq!(nat.mapped_prefix, "203.0.113.10/32");
        assert_eq!(nat.evidence_refs.len(), 3);
    }
}
