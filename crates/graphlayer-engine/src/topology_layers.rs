//! Synthetic, bounded multi-layer topology projection for scale exploration.
//! The corpus is generated here so UI code only renders Rust-owned topology.
use serde_json::{json, Value};
use sha2::{Digest, Sha256};
use std::collections::{HashMap, HashSet, VecDeque};
use std::net::Ipv4Addr;

const HOST_LIMIT: usize = 10_000;
const LINK_LIMIT: usize = 20_000;
const SOURCE_LIMIT: usize = 4 << 20;

fn hash(text: &str) -> String {
    format!("{:x}", Sha256::digest(text.as_bytes()))
}
fn subnet_parts(value: &str) -> Option<(u32, u32)> {
    let (address, bits) = value.split_once('/')?;
    let address = address.parse::<Ipv4Addr>().ok()?;
    let bits = bits.parse::<u32>().ok()?;
    if bits > 32 || !address.is_private() {
        return None;
    }
    let mask = if bits == 0 {
        0
    } else {
        u32::MAX << (32 - bits)
    };
    let prefix = u32::from(address);
    (prefix & mask == prefix).then_some((prefix, mask))
}
fn only(value: &Value, allowed: &[&str]) -> bool {
    value
        .as_object()
        .is_some_and(|o| o.keys().all(|k| allowed.contains(&k.as_str())))
}
fn valid_scope_fields(row: &Value) -> bool {
    let Some(obj) = row.as_object() else {
        return false;
    };
    let (Some(vrf), Some(vdom)) = (obj.get("vrf"), obj.get("vdom")) else {
        return !obj.contains_key("vrf") && !obj.contains_key("vdom");
    };
    [vrf, vdom].iter().all(|v| {
        v.as_str().is_some_and(|s| {
            !s.is_empty()
                && s.len() <= 80
                && s.chars()
                    .all(|c| c.is_ascii_alphanumeric() || "._-".contains(c))
        })
    })
}
fn source(id: &str, label: &str, kind: &str, text: &str) -> Value {
    let lines: Vec<Value> = text
        .lines()
        .enumerate()
        .map(|(i, t)| json!({"number":i+1,"text":t}))
        .collect();
    json!({"id":id,"label":label,"kind":kind,"sha256":hash(text),"text":text,"lines":lines})
}
fn canonical_record(value: &Value) -> Result<String, String> {
    let mut row = value.clone();
    row.as_object_mut()
        .ok_or("invalid_record")?
        .remove("evidence");
    serde_json::to_string(&row).map_err(|_| "invalid_record".into())
}
fn shares_source_ref(a: &Value, b: &Value) -> bool {
    a.as_array().is_some_and(|left| {
        b.as_array().is_some_and(|right| {
            left.iter().any(|x| {
                x["source_id"] != "synthetic-records"
                    && right
                        .iter()
                        .any(|y| x["source_id"] == y["source_id"] && x["ref"] == y["ref"])
            })
        })
    })
}
fn record_lines(
    kind: u8,
    rows: &[Value],
    lines: &mut Vec<String>,
    refs: &mut Vec<(u8, usize, usize)>,
) -> Result<(), String> {
    for (index, row) in rows.iter().enumerate() {
        lines.push(canonical_record(row)?);
        refs.push((kind, index, lines.len()));
    }
    Ok(())
}
fn refev(id: &str, line: usize, digest: &str) -> Value {
    json!({"source_id":id,"ref":format!("{}:{}:L{}",id,&digest[..12],line),"line":line,"sha256":digest})
}

/// Return the same deterministic, fully synthetic 1,000-host dataset each time.
pub fn layer_sample(input: Value) -> Result<Value, String> {
    if !input.is_object() || input.as_object().is_some_and(|o| !o.is_empty()) {
        return Err("invalid_layer_request".into());
    }
    layer_sample_sized(10, 100)
}

fn layer_sample_sized(network_count: usize, hosts_per_network: usize) -> Result<Value, String> {
    let pan = "set deviceconfig system hostname scale-panos\nset network interface ethernet ethernet1/1 layer3 ip 10.20.1.254/24\nset network virtual-router default interface ethernet1/1\n";
    let forti = "config system global\n    set hostname scale-fortigate\nend\nconfig system interface\n    edit \"port1\"\n        set vdom \"root\"\n        set ip 10.20.2.254 255.255.255.0\n        set allowaccess ping\n    next\nend\n";
    // Exercise the same Rust parsers used by parse_config and retain stable source hashes.
    let pan_hash = hash(pan);
    let forti_hash = hash(forti);
    let pan_parse = crate::vendor_parsers::parse_panos_with_hash(pan, &pan_hash)?;
    let forti_parse = crate::vendor_parsers::parse_fortigate_with_hash(forti, &forti_hash)?;
    let mut sources = vec![
        source(
            "cfg-panos",
            "Synthetic PAN-OS configuration",
            "configuration",
            pan,
        ),
        source(
            "cfg-fortios",
            "Synthetic FortiOS configuration",
            "configuration",
            forti,
        ),
    ];
    let mut hosts = Vec::with_capacity(network_count * hosts_per_network);
    let mut devices = Vec::with_capacity(network_count * 4);
    let mut links = Vec::with_capacity(network_count * (hosts_per_network + 6));
    let mut networks = Vec::new();
    let zones = ["engineering", "operations", "research", "shared-services"];
    for n in 1..=network_count {
        let subnet = format!("10.20.{}.0/24", n);
        let net_id = format!("net-{n:02}");
        let zone = zones[(n - 1) % zones.len()];
        let vrf = format!("lab-vrf-{:02}", n);
        let vdom = if n == 2 { "root" } else { "campus" };
        let inv_id = format!("inventory-{n:02}");
        let first_switch_hosts = hosts_per_network / 2;
        let second_switch_hosts = hosts_per_network - first_switch_hosts;
        let host_port_offset = if hosts_per_network > 100 { 60 } else { 0 };
        let mut lines = vec![format!(
            "network {net_id} {subnet} zone {zone} gateway 10.20.{n}.253 via rtr-{n:02} vrf {vrf} vdom {vdom}"
        )];
        for h in 1..=hosts_per_network {
            let mac = format!(
                "02:20:{n:02x}:{:02x}:{:02x}:{:02x}",
                (h >> 8) & 255,
                h & 255,
                n
            );
            lines.push(format!(
                "host host-{:04} 10.20.{}.{} mac {} vlan {} port Gi1/0/{} arp 10.20.{}.{} {} vrf {} vdom {}",
                (n - 1) * hosts_per_network + h,
                n,
                h,
                mac,
                100 + n,
                if h <= first_switch_hosts {
                    h + host_port_offset
                } else {
                    h - first_switch_hosts + host_port_offset
                },
                n,
                h,
                mac,
                vrf,
                vdom
            ));
        }
        if n == 1 {
            lines.push("firewall-uplink rtr-01 Gi0/1 fw-panos ethernet1/1".into());
        }
        if n == 2 {
            lines.push("firewall-uplink rtr-02 Gi0/1 fw-fortigate port1".into());
        }
        let invtext = lines.join("\n");
        let invhash = hash(&invtext);
        sources.push(source(
            &inv_id,
            &format!("Synthetic {net_id} inventory",),
            "inventory",
            &invtext,
        ));
        let prefix = format!("10.20.{n}");
        let sw_a = format!("sw-{n:02}-a");
        let sw_b = format!("sw-{n:02}-b");
        let dist = format!("sw-{n:02}-d");
        devices.push(json!({"id":sw_a,"label":sw_a,"role":"switch","zone":zone,"network_id":net_id,"subnet":subnet,"vrf":vrf,"vdom":vdom,"host_count":first_switch_hosts,"evidence":[refev(&inv_id,1,&invhash)]}));
        devices.push(json!({"id":sw_b,"label":sw_b,"role":"switch","zone":zone,"network_id":net_id,"subnet":subnet,"vrf":vrf,"vdom":vdom,"host_count":second_switch_hosts,"evidence":[refev(&inv_id,1,&invhash)]}));
        devices.push(json!({"id":dist,"label":dist,"role":"switch","zone":zone,"network_id":net_id,"subnet":subnet,"vrf":vrf,"vdom":vdom,"host_count":0,"evidence":[refev(&inv_id,1,&invhash)]}));
        devices.push(json!({"id":format!("rtr-{n:02}"),"label":format!("Router {n:02}"),"role":"router","zone":zone,"network_id":net_id,"subnet":subnet,"ip":format!("10.20.{n}.253"),"vrf":vrf,"vdom":vdom,"host_count":0,"evidence":[refev(&inv_id,1,&invhash)]}));
        networks.push(
            json!({"id":net_id,"subnet":subnet,"zone":zone,"vrf":vrf,"vdom":vdom,"evidence":[refev(&inv_id,1,&invhash)]}),
        );
        for h in 1..=hosts_per_network {
            let ix = (n - 1) * hosts_per_network + h;
            let id = format!("host-{ix:04}");
            let ip = format!("{prefix}.{h}");
            let mac = format!(
                "02:20:{n:02x}:{:02x}:{:02x}:{:02x}",
                (h >> 8) & 255,
                h & 255,
                n
            );
            let sw = if h <= first_switch_hosts {
                &sw_a
            } else {
                &sw_b
            };
            let port = if h <= first_switch_hosts {
                h + host_port_offset
            } else {
                h - first_switch_hosts + host_port_offset
            };
            hosts.push(json!({"id":id,"label":id,"role":"host","zone":zone,"network_id":net_id,"subnet":subnet,"vrf":vrf,"vdom":vdom,"ip":ip,"mac":mac,"vlan":100+n,"port":format!("Gi1/0/{port}"),"attachment":sw,"evidence":[refev(&inv_id,h+1,&invhash)]}));
            links.push(json!({"id":format!("attach-{ix:04}"),"from":id,"to":sw,"kind":"attachment","label":format!("VLAN {}",100+n),"from_port":"eth0","to_port":format!("Gi1/0/{port}"),"vlan":100+n,"evidence":[refev(&inv_id,h+1,&invhash)]}));
        }
        let up_id = format!("uplinks-{n:02}");
        let uptext=format!("uplink {} Gi1/0/51 {} Gi1/0/1 vlan {}\nuplink {} Gi1/0/52 {} Gi1/0/2 vlan {}\nrouted-uplink {} Gi1/0/48 rtr-{:02} Gi0/0",sw_a,dist,100+n,sw_b,dist,100+n,dist,n);
        let uphash = hash(&uptext);
        sources.push(source(
            &up_id,
            &format!("Synthetic {net_id} switch uplinks"),
            "uplinks",
            &uptext,
        ));
        let ev = refev(&up_id, 1, &uphash);
        links.push(json!({"id":format!("uplink-{n:02}-a"),"from":sw_a,"to":dist,"kind":"trunk","label":"802.1Q trunk","from_port":"Gi1/0/51","to_port":"Gi1/0/1","vlan":100+n,"evidence":[ev]}));
        let ev = refev(&up_id, 2, &uphash);
        links.push(json!({"id":format!("uplink-{n:02}-b"),"from":sw_b,"to":dist,"kind":"trunk","label":"802.1Q trunk","from_port":"Gi1/0/52","to_port":"Gi1/0/2","vlan":100+n,"evidence":[ev]}));
        let ev = refev(&up_id, 3, &uphash);
        links.push(json!({"id":format!("uplink-{n:02}-router"),"from":dist,"to":format!("rtr-{n:02}"),"kind":"routed_uplink","label":"Routed uplink","from_port":"Gi1/0/48","to_port":"Gi0/0","vlan":null,"evidence":[ev]}));
        let ev = refev(&inv_id, 1, &invhash);
        links.push(json!({"id":format!("network-gateway-{n:02}"),"from":net_id,"to":format!("rtr-{n:02}"),"kind":"declared_gateway","label":"Declared synthetic gateway","evidence":[ev]}));
    }
    // Link each modeled router to a parser-backed configured firewall gateway relationship.
    for (id, net, source_id, digest, interface, config_line, inventory_id) in [
        (
            "fw-panos",
            "net-01",
            "cfg-panos",
            pan_hash.as_str(),
            "ethernet1/1",
            2,
            "inventory-01",
        ),
        (
            "fw-fortigate",
            "net-02",
            "cfg-fortios",
            forti_hash.as_str(),
            "port1",
            7,
            "inventory-02",
        ),
    ] {
        let inventory_hash = sources
            .iter()
            .find(|s| s["id"] == inventory_id)
            .and_then(|s| s["sha256"].as_str())
            .ok_or("invalid_source")?
            .to_owned();
        let node = json!({"id":id,"label":if id=="fw-panos" {"PAN-OS edge"} else {"FortiGate edge"},"role":"firewall","zone":"shared-services","network_id":net,"subnet":if net=="net-01" {"10.20.1.0/24"} else {"10.20.2.0/24"},"ip":if net=="net-01" {"10.20.1.254"} else {"10.20.2.254"},"evidence":[refev(source_id,config_line,digest)]});
        devices.push(node);
        let uplink = format!("gateway-{id}");
        links.push(json!({"id":uplink,"from":format!("rtr-{}",if net=="net-01" {"01"} else {"02"}),"to":id,"kind":"declared_uplink","label":"Declared synthetic firewall uplink","from_port":"Gi0/1","to_port":interface,"evidence":[refev(inventory_id,hosts_per_network+2,&inventory_hash),refev(source_id,config_line,digest)]}));
    }
    // Bind every normalized row to a canonical source line so evidence cannot be reassigned.
    let mut canonical_lines = Vec::new();
    let mut canonical_refs = Vec::new();
    record_lines(0, &hosts, &mut canonical_lines, &mut canonical_refs)?;
    record_lines(1, &devices, &mut canonical_lines, &mut canonical_refs)?;
    record_lines(2, &networks, &mut canonical_lines, &mut canonical_refs)?;
    record_lines(3, &links, &mut canonical_lines, &mut canonical_refs)?;
    let canonical_text = canonical_lines.join("\n");
    let canonical_hash = hash(&canonical_text);
    sources.push(source(
        "synthetic-records",
        "Canonical synthetic topology records",
        "synthetic_records",
        &canonical_text,
    ));
    for (kind, index, line) in canonical_refs {
        let ev = refev("synthetic-records", line, &canonical_hash);
        let row = match kind {
            0 => &mut hosts[index],
            1 => &mut devices[index],
            2 => &mut networks[index],
            _ => &mut links[index],
        };
        row["evidence"]
            .as_array_mut()
            .ok_or("invalid_record")?
            .push(ev);
    }
    // Device nodes are declared in the source inventory, keeping attachments and links auditable.
    let model = json!({"schema_version":1,"synthetic":true,"label":"Synthetic 1,000-host campus topology","hosts":hosts,"devices":devices,"networks":networks,"links":links,"zones":zones,"sources":sources,"parsed_configs":[{"source_id":"cfg-panos","vendor":"paloalto-panos","result":pan_parse},{"source_id":"cfg-fortios","vendor":"fortinet-fortigate","result":forti_parse}]});
    // Validate source sizes and graph references before exposing the fixture.
    validate_dataset(&model)?;
    Ok(json!({"dataset":model}))
}

fn validate_dataset(d: &Value) -> Result<(), String> {
    if !only(
        d,
        &[
            "schema_version",
            "synthetic",
            "label",
            "hosts",
            "devices",
            "networks",
            "links",
            "zones",
            "sources",
            "parsed_configs",
        ],
    ) {
        return Err("invalid_dataset".into());
    }
    if d["schema_version"].as_u64() != Some(1) || d["synthetic"].as_bool() != Some(true) {
        return Err("invalid_dataset".into());
    }
    let hosts = d["hosts"].as_array().ok_or("invalid_dataset")?;
    let links = d["links"].as_array().ok_or("invalid_dataset")?;
    let sources = d["sources"].as_array().ok_or("invalid_dataset")?;
    let devices = d["devices"].as_array().ok_or("invalid_dataset")?;
    let networks = d["networks"].as_array().ok_or("invalid_dataset")?;
    let zones = d["zones"].as_array().ok_or("invalid_dataset")?;
    if hosts.len() > HOST_LIMIT
        || links.len() > LINK_LIMIT
        || devices.len() > 200
        || networks.len() > 100
        || zones.len() > 100
        || sources.len() > 100
    {
        return Err("dataset_limit".into());
    }
    let mut zone_ids = HashSet::new();
    for zone in zones {
        let zone = zone.as_str().ok_or("invalid_zone")?;
        if zone.is_empty() || zone.len() > 80 || !zone_ids.insert(zone) {
            return Err("invalid_zone".into());
        }
    }
    let mut subnet_index = HashMap::new();
    for network in networks {
        let id = network["id"].as_str().ok_or("invalid_network")?;
        let subnet = network["subnet"].as_str().ok_or("invalid_network")?;
        let zone = network["zone"].as_str().ok_or("invalid_network")?;
        let (prefix, mask) = subnet_parts(subnet).ok_or("invalid_network")?;
        if !zone_ids.contains(zone)
            || subnet_index
                .insert(id, (subnet, zone, prefix, mask))
                .is_some()
        {
            return Err("invalid_network".into());
        }
    }
    let bytes = serde_json::to_vec(&d["sources"]).map_err(|_| "invalid_dataset")?;
    if bytes.len() > SOURCE_LIMIT {
        return Err("source_limit".into());
    }
    let mut ids = HashSet::new();
    let mut ips = HashSet::new();
    let mut macs = HashSet::new();
    for net in d["networks"].as_array().ok_or("invalid_dataset")? {
        if !only(net, &["id", "subnet", "zone", "vrf", "vdom", "evidence"]) {
            return Err("invalid_dataset".into());
        }
        let id = net["id"].as_str().ok_or("invalid_dataset")?;
        if !valid_scope_fields(net) {
            return Err("invalid_scope".into());
        }
        if !ids.insert(id.to_owned()) {
            return Err("duplicate_node".into());
        }
    }
    for node in devices {
        if !only(
            node,
            &[
                "id",
                "label",
                "role",
                "zone",
                "network_id",
                "subnet",
                "ip",
                "vrf",
                "vdom",
                "host_count",
                "evidence",
            ],
        ) {
            return Err("invalid_dataset".into());
        }
        let id = node["id"].as_str().ok_or("invalid_dataset")?;
        if !valid_scope_fields(node) {
            return Err("invalid_scope".into());
        }
        if !ids.insert(id.to_owned()) {
            return Err("duplicate_node".into());
        }
        if let Some(ip) = node["ip"].as_str() {
            let parsed = ip.parse::<Ipv4Addr>().map_err(|_| "invalid_device_ip")?;
            if !parsed.is_private() || !ips.insert(ip.to_owned()) {
                return Err("invalid_device_ip".into());
            }
        }
        if matches!(node["role"].as_str(), Some("switch" | "router")) {
            let net = networks
                .iter()
                .find(|n| n["id"] == node["network_id"])
                .ok_or("invalid_network")?;
            if node["vrf"] != net["vrf"] || node["vdom"] != net["vdom"] {
                return Err("invalid_scope".into());
            }
        }
    }
    // Keep endpoint lookups constant-time as host and link limits grow. These
    // references borrow the validated input and do not duplicate any records.
    let host_by_id: HashMap<&str, &Value> = hosts
        .iter()
        .filter_map(|host| Some((host["id"].as_str()?, host)))
        .collect();
    let device_by_id: HashMap<&str, &Value> = devices
        .iter()
        .filter_map(|device| Some((device["id"].as_str()?, device)))
        .collect();
    for h in hosts {
        if !only(
            h,
            &[
                "id",
                "label",
                "role",
                "zone",
                "network_id",
                "subnet",
                "ip",
                "mac",
                "vlan",
                "port",
                "attachment",
                "vrf",
                "vdom",
                "evidence",
            ],
        ) {
            return Err("invalid_host".into());
        }
        let id = h["id"].as_str().ok_or("invalid_dataset")?;
        if !valid_scope_fields(h) {
            return Err("invalid_scope".into());
        }
        if !ids.insert(id.to_owned()) {
            return Err("duplicate_host".into());
        }
        let ip = h["ip"].as_str().ok_or("invalid_dataset")?;
        let parsed = ip.parse::<Ipv4Addr>().map_err(|_| "invalid_host_ip")?;
        if !parsed.is_private() || !ips.insert(ip.to_owned()) {
            return Err("invalid_host_ip".into());
        }
        let network = h["network_id"].as_str().ok_or("invalid_host_network")?;
        let (subnet, zone, prefix, mask) =
            subnet_index.get(network).ok_or("invalid_host_network")?;
        let network_row = networks
            .iter()
            .find(|n| n["id"] == network)
            .ok_or("invalid_host_network")?;
        if h["role"] != "host"
            || h["subnet"] != *subnet
            || h["zone"] != *zone
            || h["vrf"] != network_row["vrf"]
            || h["vdom"] != network_row["vdom"]
            || u32::from(parsed) & *mask != *prefix
        {
            return Err("invalid_host_network".into());
        }
        let mac = h["mac"].as_str().ok_or("invalid_dataset")?;
        if mac.split(':').any(|part| part.len() != 2) {
            return Err("invalid_mac".into());
        }
        let octets: Vec<u8> = mac
            .split(':')
            .map(|part| u8::from_str_radix(part, 16).map_err(|_| "invalid_mac"))
            .collect::<Result<_, _>>()?;
        if octets.len() != 6 || octets[0] & 2 == 0 || octets[0] & 1 != 0 || !macs.insert(octets) {
            return Err("invalid_mac".into());
        }
    }
    let mut source_lines = HashMap::new();
    for s in sources {
        if !only(s, &["id", "label", "kind", "sha256", "text", "lines"]) {
            return Err("invalid_source".into());
        }
        let id = s["id"].as_str().ok_or("invalid_source")?;
        let lines = s["lines"].as_array().ok_or("invalid_source")?;
        let text = s["text"].as_str().ok_or("invalid_source")?;
        let actual: Vec<&str> = text.lines().collect();
        let digest = s["sha256"].as_str().ok_or("invalid_source")?;
        if source_lines
            .insert(id.to_owned(), (actual, digest, text))
            .is_some()
            || digest != hash(text)
            || lines.len() != text.lines().count()
        {
            return Err("invalid_source".into());
        }
        for (i, (line, expected)) in lines.iter().zip(text.lines()).enumerate() {
            if !only(line, &["number", "text"])
                || line["number"].as_u64() != Some((i + 1) as u64)
                || line["text"].as_str() != Some(expected)
            {
                return Err("invalid_source".into());
            }
        }
    }
    let configs = d["parsed_configs"].as_array().ok_or("invalid_dataset")?;
    for cfg in configs {
        let sid = cfg["source_id"].as_str().ok_or("invalid_config")?;
        let (_, digest, text) = source_lines.get(sid).ok_or("invalid_config")?;
        let vendor = cfg["vendor"].as_str().ok_or("invalid_config")?;
        let parsed = match vendor {
            "paloalto-panos" => crate::vendor_parsers::parse_panos_with_hash(text, digest)?,
            "fortinet-fortigate" => crate::vendor_parsers::parse_fortigate_with_hash(text, digest)?,
            _ => return Err("invalid_config".into()),
        };
        if parsed != cfg["result"] {
            return Err("config_parse_mismatch".into());
        }
    }
    if configs.len() != 2 {
        return Err("invalid_config".into());
    }
    for (sid, expected) in [
        ("cfg-panos", "10.20.1.254/24"),
        ("cfg-fortios", "10.20.2.254/24"),
    ] {
        let cfg = configs
            .iter()
            .find(|c| c["source_id"] == sid)
            .ok_or("invalid_config")?;
        if !cfg["result"]["path"]["interfaces"]
            .as_array()
            .ok_or("invalid_config")?
            .iter()
            .any(|i| i["address"] == expected)
        {
            return Err("gateway_config_mismatch".into());
        }
    }
    for row in hosts.iter().chain(devices).chain(networks) {
        let evidence = row["evidence"].as_array().ok_or("invalid_evidence")?;
        if evidence.is_empty() {
            return Err("invalid_evidence".into());
        }
        for ev in evidence {
            if !only(ev, &["source_id", "ref", "line", "sha256"]) {
                return Err("invalid_evidence".into());
            }
            let sid = ev["source_id"].as_str().ok_or("invalid_evidence")?;
            let line = ev["line"].as_u64().ok_or("invalid_evidence")? as usize;
            let Some((source_rows, digest, _)) = source_lines.get(sid) else {
                return Err("invalid_evidence".into());
            };
            if line == 0
                || line > source_rows.len()
                || ev["sha256"].as_str() != Some(digest)
                || ev["ref"].as_str() != Some(format!("{sid}:{}:L{line}", &digest[..12]).as_str())
            {
                return Err("invalid_evidence".into());
            }
        }
    }
    if sources
        .iter()
        .find(|s| s["id"] == "synthetic-records")
        .is_none()
    {
        return Err("missing_canonical_records".into());
    }
    for row in hosts.iter().chain(devices).chain(networks).chain(links) {
        let canonical = canonical_record(row)?;
        let mut matched = false;
        for ev in row["evidence"].as_array().ok_or("invalid_evidence")? {
            if ev["source_id"] == "synthetic-records" {
                let line = ev["line"].as_u64().ok_or("invalid_evidence")? as usize;
                let (source_rows, _, _) = source_lines
                    .get("synthetic-records")
                    .ok_or("invalid_evidence")?;
                if source_rows.get(line.saturating_sub(1)).copied() != Some(canonical.as_str()) {
                    return Err("record_evidence_mismatch".into());
                }
                matched = true
            }
        }
        if !matched {
            return Err("missing_canonical_record_ref".into());
        }
    }
    let mut network_vlan = HashMap::new();
    let mut used_ports = HashSet::new();
    for h in hosts {
        let network = h["network_id"].as_str().ok_or("invalid_host")?;
        let vlan = h["vlan"].as_u64().ok_or("invalid_host")?;
        if network_vlan
            .insert(network.to_owned(), vlan)
            .is_some_and(|old| old != vlan)
        {
            return Err("inconsistent_network_vlan".into());
        }
        let switch = h["attachment"].as_str().ok_or("invalid_attachment")?;
        let device = device_by_id
            .get(switch)
            .filter(|n| {
                n["role"] == "switch"
                    && n["network_id"] == network
                    && n["zone"] == h["zone"]
                    && n["vrf"] == h["vrf"]
                    && n["vdom"] == h["vdom"]
            })
            .ok_or("invalid_attachment")?;
        let port = h["port"].as_str().ok_or("invalid_attachment")?;
        if !used_ports.insert((
            device["id"].as_str().unwrap_or("").to_owned(),
            port.to_owned(),
        )) {
            return Err("duplicate_switch_port".into());
        }
    }
    let mut link_ids = HashSet::new();
    for l in links {
        if !only(
            l,
            &[
                "id",
                "from",
                "to",
                "kind",
                "label",
                "from_port",
                "to_port",
                "vlan",
                "evidence",
            ],
        ) {
            return Err("invalid_link".into());
        }
        let id = l["id"].as_str().ok_or("invalid_link")?;
        if !link_ids.insert(id.to_owned())
            || l["evidence"].as_array().is_none()
            || !matches!(
                l["kind"].as_str(),
                Some(
                    "attachment"
                        | "trunk"
                        | "routed_uplink"
                        | "declared_gateway"
                        | "declared_uplink"
                )
            )
        {
            return Err("invalid_link".into());
        }
        for end in ["from", "to"] {
            if !ids.contains(l[end].as_str().ok_or("invalid_link")?) {
                return Err("dangling_link".into());
            }
        }
    }
    for l in links {
        let evidence = l["evidence"].as_array().ok_or("invalid_evidence")?;
        if evidence.is_empty() {
            return Err("invalid_evidence".into());
        }
        for ev in evidence {
            if !only(ev, &["source_id", "ref", "line", "sha256"]) {
                return Err("invalid_evidence".into());
            }
            let sid = ev["source_id"].as_str().ok_or("invalid_evidence")?;
            let line = ev["line"].as_u64().ok_or("invalid_evidence")? as usize;
            let Some((source_rows, digest, _)) = source_lines.get(sid) else {
                return Err("invalid_evidence".into());
            };
            if line == 0
                || line > source_rows.len()
                || ev["sha256"].as_str() != Some(digest)
                || ev["ref"].as_str() != Some(format!("{sid}:{}:L{line}", &digest[..12]).as_str())
            {
                return Err("invalid_evidence".into());
            }
        }
        if l["kind"] == "attachment" {
            let host = host_by_id
                .get(l["from"].as_str().ok_or("invalid_attachment")?)
                .ok_or("invalid_attachment")?;
            let sw = device_by_id
                .get(l["to"].as_str().ok_or("invalid_attachment")?)
                .filter(|n| {
                    n["role"] == "switch"
                        && n["network_id"] == host["network_id"]
                        && n["zone"] == host["zone"]
                })
                .ok_or("invalid_attachment")?;
            if host["attachment"] != l["to"]
                || host["port"] != l["to_port"]
                || host["vlan"] != l["vlan"]
                || !shares_source_ref(&l["evidence"], &host["evidence"])
                || l["from_port"] != "eth0"
                || sw["network_id"] != host["network_id"]
            {
                return Err("invalid_attachment".into());
            }
        }
        if l["kind"] == "trunk" {
            let sw_a = device_by_id
                .get(l["from"].as_str().ok_or("invalid_link")?)
                .ok_or("invalid_link")?;
            let sw_b = device_by_id
                .get(l["to"].as_str().ok_or("invalid_link")?)
                .ok_or("invalid_link")?;
            let vlan = l["vlan"].as_u64().ok_or("invalid_link")?;
            if sw_a["role"] != "switch"
                || sw_b["role"] != "switch"
                || sw_a["network_id"] != sw_b["network_id"]
                || sw_a["zone"] != sw_b["zone"]
                || network_vlan.get(sw_a["network_id"].as_str().unwrap_or("")) != Some(&vlan)
            {
                return Err("invalid_link".into());
            }
            for (node, port) in [
                (sw_a, l["from_port"].as_str().ok_or("invalid_link")?),
                (sw_b, l["to_port"].as_str().ok_or("invalid_link")?),
            ] {
                if !used_ports.insert((
                    node["id"].as_str().unwrap_or("").to_owned(),
                    port.to_owned(),
                )) {
                    return Err("duplicate_switch_port".into());
                }
            }
            let ev = &l["evidence"][0];
            let (source_rows, _, _) = source_lines
                .get(ev["source_id"].as_str().unwrap())
                .ok_or("invalid_evidence")?;
            let line = source_rows
                .get(ev["line"].as_u64().unwrap() as usize - 1)
                .ok_or("invalid_evidence")?;
            let expected = format!(
                "uplink {} {} {} {} vlan {}",
                l["from"].as_str().unwrap_or(""),
                l["from_port"].as_str().unwrap_or(""),
                l["to"].as_str().unwrap_or(""),
                l["to_port"].as_str().unwrap_or(""),
                vlan
            );
            if *line != expected {
                return Err("invalid_link".into());
            }
        }
        if l["kind"] == "routed_uplink" {
            let sw = device_by_id
                .get(l["from"].as_str().ok_or("invalid_link")?)
                .ok_or("invalid_link")?;
            let router = device_by_id
                .get(l["to"].as_str().ok_or("invalid_link")?)
                .ok_or("invalid_link")?;
            if sw["role"] != "switch"
                || router["role"] != "router"
                || sw["network_id"] != router["network_id"]
                || sw["zone"] != router["zone"]
                || !used_ports.insert((
                    sw["id"].as_str().unwrap_or("").to_owned(),
                    l["from_port"].as_str().ok_or("invalid_link")?.to_owned(),
                ))
            {
                return Err("invalid_link".into());
            }
            let ev = &l["evidence"][0];
            let (source_rows, _, _) = source_lines
                .get(ev["source_id"].as_str().unwrap())
                .ok_or("invalid_evidence")?;
            let line = source_rows
                .get(ev["line"].as_u64().unwrap() as usize - 1)
                .ok_or("invalid_evidence")?;
            let expected = format!(
                "routed-uplink {} {} {} {}",
                l["from"].as_str().unwrap_or(""),
                l["from_port"].as_str().unwrap_or(""),
                l["to"].as_str().unwrap_or(""),
                l["to_port"].as_str().unwrap_or("")
            );
            if *line != expected {
                return Err("invalid_link".into());
            }
        }
        if l["kind"] == "declared_gateway" {
            let ev = &l["evidence"][0];
            let (source_rows, _, _) = source_lines
                .get(ev["source_id"].as_str().unwrap())
                .ok_or("invalid_evidence")?;
            let line = source_rows
                .get(ev["line"].as_u64().unwrap() as usize - 1)
                .ok_or("invalid_evidence")?;
            let net = networks
                .iter()
                .find(|n| n["id"] == l["from"])
                .ok_or("invalid_link")?;
            let router = device_by_id
                .get(l["to"].as_str().ok_or("invalid_link")?)
                .filter(|n| {
                    n["role"] == "router"
                        && n["network_id"] == l["from"]
                        && n["zone"] == net["zone"]
                })
                .ok_or("invalid_link")?;
            let expected = format!(
                "network {} {} zone {} gateway {} via {}{}",
                net["id"].as_str().ok_or("invalid_link")?,
                net["subnet"].as_str().ok_or("invalid_link")?,
                net["zone"].as_str().ok_or("invalid_link")?,
                router["ip"].as_str().ok_or("invalid_link")?,
                router["id"].as_str().ok_or("invalid_link")?,
                match (net["vrf"].as_str(), net["vdom"].as_str()) {
                    (Some(vrf), Some(vdom)) => format!(" vrf {vrf} vdom {vdom}"),
                    _ => String::new(),
                }
            );
            if line != &expected {
                return Err("invalid_link".into());
            }
        }
        if l["kind"] == "declared_uplink" {
            let router = device_by_id
                .get(l["from"].as_str().ok_or("invalid_gateway")?)
                .filter(|n| n["role"] == "router")
                .ok_or("invalid_gateway")?;
            let fw = device_by_id
                .get(l["to"].as_str().ok_or("invalid_gateway")?)
                .filter(|n| n["role"] == "firewall")
                .ok_or("invalid_gateway")?;
            let ev = &l["evidence"][0];
            let sid = ev["source_id"].as_str().ok_or("invalid_evidence")?;
            let (source_rows, _, _) = source_lines.get(sid).ok_or("invalid_evidence")?;
            let record = format!(
                "firewall-uplink {} {} {} {}",
                l["from"].as_str().unwrap_or(""),
                l["from_port"].as_str().unwrap_or(""),
                l["to"].as_str().unwrap_or(""),
                l["to_port"].as_str().unwrap_or("")
            );
            if router["network_id"] != fw["network_id"]
                || source_rows
                    .get(ev["line"].as_u64().unwrap_or(0).saturating_sub(1) as usize)
                    .copied()
                    != Some(record.as_str())
                || evidence.len() != 3
                || !configs.iter().any(|c| {
                    c["source_id"] == evidence[1]["source_id"]
                        && c["result"]["path"]["interfaces"]
                            .as_array()
                            .is_some_and(|ifs| {
                                ifs.iter().any(|i| {
                                    i["name"] == l["to_port"]
                                        && i["address"].as_str().and_then(|a| a.split('/').next())
                                            == fw["ip"].as_str()
                                })
                            })
                })
            {
                return Err("invalid_gateway".into());
            }
        }
    }
    for h in hosts {
        if !ids.contains(h["attachment"].as_str().ok_or("invalid_attachment")?)
            || !h["vlan"].as_u64().is_some_and(|v| (1..=4094).contains(&v))
            || h["port"].as_str().is_none()
        {
            return Err("invalid_attachment".into());
        }
        let ev = &h["evidence"][0];
        let (source_rows, _, _) = source_lines
            .get(ev["source_id"].as_str().ok_or("invalid_evidence")?)
            .ok_or("invalid_evidence")?;
        let line = source_rows
            .get(ev["line"].as_u64().ok_or("invalid_evidence")? as usize - 1)
            .ok_or("invalid_evidence")?;
        let expected = format!(
            "host {} {} mac {} vlan {} port {} arp {} {}{}",
            h["id"].as_str().ok_or("invalid_host")?,
            h["ip"].as_str().ok_or("invalid_host")?,
            h["mac"].as_str().ok_or("invalid_host")?,
            h["vlan"].as_u64().ok_or("invalid_host")?,
            h["port"].as_str().ok_or("invalid_host")?,
            h["ip"].as_str().unwrap(),
            h["mac"].as_str().unwrap(),
            match (h["vrf"].as_str(), h["vdom"].as_str()) {
                (Some(vrf), Some(vdom)) => format!(" vrf {vrf} vdom {vdom}"),
                _ => String::new(),
            }
        );
        if line != &expected {
            return Err("host_evidence_mismatch".into());
        }
    }
    Ok(())
}

pub fn project_layers(input: Value) -> Result<Value, String> {
    if !input.as_object().is_some_and(|o| {
        o.keys().all(|k| {
            [
                "dataset",
                "view",
                "network_id",
                "zone",
                "search",
                "vrf",
                "vdom",
                "offset",
                "limit",
                "focus_host_id",
                "source_host_id",
                "destination_host_id",
            ]
            .contains(&k.as_str())
        })
    }) {
        return Err("invalid_layer_request".into());
    }
    for key in [
        "network_id",
        "zone",
        "search",
        "vrf",
        "vdom",
        "source_host_id",
        "destination_host_id",
        "focus_host_id",
    ] {
        if input.get(key).is_some_and(|v| !v.is_string()) {
            return Err("invalid_layer_request".into());
        }
    }
    for key in ["vrf", "vdom"] {
        if input
            .get(key)
            .and_then(Value::as_str)
            .is_some_and(|v| v.len() > 80 || v.trim().is_empty() && !v.is_empty())
        {
            return Err("invalid_layer_request".into());
        }
    }
    for key in ["offset", "limit"] {
        if input.get(key).is_some_and(|v| !v.is_u64()) {
            return Err("invalid_layer_request".into());
        }
    }
    let d = input.get("dataset").ok_or("invalid_layer_request")?;
    validate_dataset(d)?;
    let view = input["view"].as_str().ok_or("invalid_layer_request")?;
    if !["l2", "l3", "zone"].contains(&view) {
        return Err("invalid_layer_request".into());
    }
    let hosts = d["hosts"].as_array().ok_or("invalid_dataset")?;
    let links = d["links"].as_array().ok_or("invalid_dataset")?;
    let sources = d["sources"].as_array().cloned().unwrap_or_default();
    let search = input["search"].as_str().unwrap_or("").to_lowercase();
    let network = input["network_id"].as_str().unwrap_or("");
    let zone = input["zone"].as_str().unwrap_or("");
    let vrf = input["vrf"].as_str().unwrap_or("").trim();
    let vdom = input["vdom"].as_str().unwrap_or("").trim();
    let mut vrfs: Vec<String> = hosts
        .iter()
        .chain(d["devices"].as_array().unwrap())
        .chain(d["networks"].as_array().unwrap())
        .filter_map(|n| n["vrf"].as_str().map(str::to_owned))
        .collect();
    vrfs.sort();
    vrfs.dedup();
    let mut vdoms: Vec<String> = hosts
        .iter()
        .chain(d["devices"].as_array().unwrap())
        .chain(d["networks"].as_array().unwrap())
        .filter_map(|n| n["vdom"].as_str().map(str::to_owned))
        .collect();
    vdoms.sort();
    vdoms.dedup();
    let host_matches_search = |h: &Value| {
        search.is_empty()
            || [
                "id",
                "label",
                "role",
                "ip",
                "address",
                "mac",
                "mac_address",
                "network_id",
                "network",
                "subnet",
                "zone",
                "vrf",
                "vdom",
                "context",
                "vlan",
                "port",
                "interface",
                "attachment",
            ]
            .iter()
            .filter_map(|key| h.get(*key))
            .filter_map(Value::as_str)
            .any(|value| value.to_lowercase().contains(&search))
            || h.get("vlan")
                .is_some_and(|value| value.to_string().to_lowercase().contains(&search))
    };
    let host_matches = |h: &Value, omit_network: bool, omit_zone: bool| {
        (omit_network || network.is_empty() || h["network_id"] == network)
            && (omit_zone || zone.is_empty() || h["zone"] == zone)
            && (vrf.is_empty() || h["vrf"].as_str() == Some(vrf))
            && (vdom.is_empty() || h["vdom"].as_str() == Some(vdom))
            && host_matches_search(h)
    };
    let matching: Vec<&Value> = hosts
        .iter()
        .filter(|h| host_matches(h, false, false))
        .collect();
    let selected_network_ids: HashSet<String> = matching
        .iter()
        .filter_map(|h| h["network_id"].as_str().map(str::to_owned))
        .collect();
    let total = matching.len();
    let limit = input["limit"].as_u64().unwrap_or(100).clamp(1, 100) as usize;
    let requested_offset = input["offset"].as_u64().unwrap_or(0).min(10000) as usize;
    let page_count = total.div_ceil(limit);
    let last_offset = page_count.saturating_sub(1) * limit;
    let clamped_offset = if total == 0 {
        0
    } else if requested_offset >= total {
        last_offset
    } else {
        requested_offset
    };
    let focus_host_id = input["focus_host_id"].as_str().unwrap_or("");
    let focus_index = if focus_host_id.is_empty() {
        None
    } else {
        matching.iter().position(|h| h["id"] == focus_host_id)
    };
    let offset = focus_index
        .map(|index| index / limit * limit)
        .unwrap_or(clamped_offset);
    let focus_status = if focus_host_id.is_empty() {
        "none"
    } else if focus_index.is_some() {
        "visible"
    } else if hosts.iter().any(|h| h["id"] == focus_host_id) {
        "outside_filters"
    } else {
        "not_found"
    };
    let page_index = if total == 0 { 0 } else { offset / limit + 1 };
    let mut network_facets: Vec<Value> = hosts
        .iter()
        .filter(|h| host_matches(h, true, false))
        .filter_map(|h| h["network_id"].as_str())
        .fold(HashMap::<String, usize>::new(), |mut counts, value| {
            *counts.entry(value.to_owned()).or_default() += 1;
            counts
        })
        .into_iter()
        .map(|(value, count)| json!({"value":value,"label":value,"count":count}))
        .collect();
    network_facets.sort_by(|a, b| a["value"].as_str().cmp(&b["value"].as_str()));
    let mut zone_facets: Vec<Value> = hosts
        .iter()
        .filter(|h| host_matches(h, false, true))
        .filter_map(|h| h["zone"].as_str())
        .fold(HashMap::<String, usize>::new(), |mut counts, value| {
            *counts.entry(value.to_owned()).or_default() += 1;
            counts
        })
        .into_iter()
        .map(|(value, count)| json!({"value":value,"label":value,"count":count}))
        .collect();
    zone_facets.sort_by(|a, b| a["value"].as_str().cmp(&b["value"].as_str()));
    let page: Vec<Value> = matching
        .iter()
        .skip(offset)
        .take(limit)
        .map(|h| (*h).clone())
        .collect();
    let mut nodes: Vec<Value> = Vec::new();
    let mut projected_links = Vec::new();
    if view == "l2" {
        let requested_target = if !network.is_empty() {
            Some(network)
        } else if let Some(focused) = focus_index.and_then(|index| matching.get(index)) {
            focused["network_id"].as_str()
        } else {
            input["source_host_id"]
                .as_str()
                .and_then(|id| {
                    matching
                        .iter()
                        .find(|h| h["id"] == id)
                        .map(|h| h["network_id"].as_str().unwrap_or(""))
                })
                .or_else(|| {
                    page.first()
                        .or_else(|| matching.first().copied())
                        .map(|h| h["network_id"].as_str().unwrap_or(""))
                })
        };
        let target = requested_target
            .filter(|target| selected_network_ids.contains(*target))
            .unwrap_or("");
        for device in d["devices"].as_array().unwrap() {
            if !target.is_empty() && device["network_id"] == target && device["role"] == "switch" {
                nodes.push(device.clone());
            }
        }
        nodes.extend(
            d["devices"]
                .as_array()
                .unwrap()
                .iter()
                .filter(|n| {
                    !target.is_empty() && n["network_id"] == target && n["role"] == "router"
                })
                .cloned(),
        );
        nodes.extend(
            page.iter()
                .filter(|h| !target.is_empty() && h["network_id"] == target)
                .cloned(),
        );
        let node_ids: HashSet<String> = nodes
            .iter()
            .filter_map(|n| n["id"].as_str().map(str::to_owned))
            .collect();
        for l in links {
            if node_ids.contains(l["from"].as_str().unwrap_or(""))
                && node_ids.contains(l["to"].as_str().unwrap_or(""))
                && l["kind"] != "routed_uplink"
            {
                projected_links.push(l.clone());
            }
        }
    } else if view == "l3" {
        let selected: HashSet<String> = d["networks"]
            .as_array()
            .unwrap()
            .iter()
            .filter(|n| {
                (network.is_empty() || n["id"] == network)
                    && selected_network_ids.contains(n["id"].as_str().unwrap_or(""))
                    && (zone.is_empty() || n["zone"] == zone)
                    && (vrf.is_empty() || n["vrf"].as_str() == Some(vrf))
                    && (vdom.is_empty() || n["vdom"].as_str() == Some(vdom))
            })
            .filter_map(|n| n["id"].as_str().map(str::to_owned))
            .collect();
        for n in d["networks"]
            .as_array()
            .unwrap_or(&vec![])
            .iter()
            .filter(|n| selected.contains(n["id"].as_str().unwrap_or("")))
        {
            let mut projected = n.clone();
            projected["label"] = n["subnet"].clone();
            projected["role"] = json!("network");
            projected["network_id"] = n["id"].clone();
            projected["host_count"] = json!(matching
                .iter()
                .filter(|h| h["network_id"] == n["id"])
                .count());
            nodes.push(projected);
        }
        nodes.extend(
            d["devices"]
                .as_array()
                .unwrap()
                .iter()
                .filter(|n| {
                    (n["role"] == "router" || n["role"] == "firewall")
                        && selected_network_ids.contains(n["network_id"].as_str().unwrap_or(""))
                        && (zone.is_empty() || n["zone"] == zone)
                })
                .cloned(),
        );
        let node_ids: HashSet<String> = nodes
            .iter()
            .filter_map(|n| n["id"].as_str().map(str::to_owned))
            .collect();
        for l in links.iter().filter(|l| {
            (l["kind"] == "declared_gateway" || l["kind"] == "declared_uplink")
                && node_ids.contains(l["from"].as_str().unwrap_or(""))
                && node_ids.contains(l["to"].as_str().unwrap_or(""))
        }) {
            projected_links.push(l.clone());
        }
    } else {
        let mut selected_zones: HashSet<String> = d["networks"]
            .as_array()
            .unwrap()
            .iter()
            .filter(|n| selected_network_ids.contains(n["id"].as_str().unwrap_or("")))
            .filter_map(|n| n["zone"].as_str().map(str::to_owned))
            .collect();
        // Explicit declared uplinks may expose the connected firewall zone,
        // but only when their router belongs to the selected network context.
        for link in links.iter().filter(|l| l["kind"] == "declared_uplink") {
            if let Some(router) = d["devices"].as_array().unwrap().iter().find(|n| {
                n["id"] == link["from"]
                    && selected_network_ids.contains(n["network_id"].as_str().unwrap_or(""))
            }) {
                if zone.is_empty() || router["zone"] == zone {
                    if let Some(fw) = d["devices"]
                        .as_array()
                        .unwrap()
                        .iter()
                        .find(|n| n["id"] == link["to"])
                    {
                        if let Some(z) = fw["zone"].as_str() {
                            selected_zones.insert(z.to_owned());
                        }
                    }
                }
            }
        }
        if !zone.is_empty() {
            selected_zones.retain(|z| {
                z == zone
                    || d["devices"].as_array().unwrap().iter().any(|n| {
                        n["role"] == "firewall"
                            && n["zone"].as_str() == Some(z.as_str())
                            && selected_network_ids.contains(n["network_id"].as_str().unwrap_or(""))
                    })
            });
        }
        for z in d["zones"].as_array().unwrap() {
            let z = z.as_str().unwrap_or("");
            if selected_zones.contains(z) {
                let mut evidence: Vec<Value> = d["networks"]
                    .as_array()
                    .unwrap()
                    .iter()
                    .chain(d["devices"].as_array().unwrap())
                    .filter(|n| {
                        n["zone"] == z
                            && selected_network_ids.contains(
                                n["network_id"]
                                    .as_str()
                                    .or_else(|| n["id"].as_str())
                                    .unwrap_or(""),
                            )
                    })
                    .flat_map(|n| n["evidence"].as_array().cloned().unwrap_or_default())
                    .collect();
                evidence.extend(
                    matching
                        .iter()
                        .filter(|h| h["zone"] == z)
                        .flat_map(|h| h["evidence"].as_array().cloned().unwrap_or_default()),
                );
                nodes.push(json!({"id":format!("zone-{z}"),"label":z,"role":"zone","zone":z,"host_count":matching.iter().filter(|h|h["zone"]==z).count(),"evidence":evidence}));
            }
        }
        // Zone links are emitted only when a parser-backed gateway explicitly joins two zones.
        let zone_by_node: HashMap<String, String> = d["networks"]
            .as_array()
            .unwrap()
            .iter()
            .chain(d["devices"].as_array().unwrap())
            .filter_map(|n| Some((n["id"].as_str()?.to_owned(), n["zone"].as_str()?.to_owned())))
            .collect();
        let mut boundaries = HashSet::new();
        for l in links.iter().filter(|l| l["kind"] == "declared_uplink") {
            let (Some(a), Some(b)) = (
                zone_by_node.get(l["from"].as_str().unwrap_or("")),
                zone_by_node.get(l["to"].as_str().unwrap_or("")),
            ) else {
                continue;
            };
            let source_selected = d["devices"].as_array().unwrap().iter().any(|n| {
                n["id"] == l["from"]
                    && selected_network_ids.contains(n["network_id"].as_str().unwrap_or(""))
            });
            if source_selected
                && a != b
                && selected_zones.contains(a)
                && selected_zones.contains(b)
                && boundaries.insert(format!("{a}:{b}"))
            {
                projected_links.push(json!({"id":format!("boundary-{a}-{b}"),"from":format!("zone-{a}"),"to":format!("zone-{b}"),"kind":"configured_boundary","label":"Declared synthetic gateway boundary","evidence":l["evidence"]}));
            }
        }
    }
    let path_result = if let (Some(src), Some(dst)) = (
        input["source_host_id"].as_str(),
        input["destination_host_id"].as_str(),
    ) {
        if !src.is_empty() && !dst.is_empty() {
            // Network, zone and search narrow the map; a trace still evaluates
            // its explicit endpoints. Declared VRF/VDOM context is the boundary
            // that must not be bypassed while adding a selected path.
            let endpoint_in_scope = |id: &str| {
                hosts.iter().find(|h| h["id"] == id).is_none_or(|h| {
                    (vrf.is_empty() || h["vrf"] == vrf) && (vdom.is_empty() || h["vdom"] == vdom)
                })
            };
            if endpoint_in_scope(src) && endpoint_in_scope(dst) {
                Some(path(hosts, links, src, dst))
            } else {
                Some(
                    json!({"status":"indeterminate","reason":"outside_selected_context","hops":[],"links":[],"evidence":[]}),
                )
            }
        } else {
            None
        }
    } else {
        None
    };
    if view == "l2" {
        if let Some(selected) = path_result
            .as_ref()
            .filter(|p| p["status"] == "modeled_l2_path")
        {
            let mut node_ids: HashSet<String> = nodes
                .iter()
                .filter_map(|n| n["id"].as_str().map(str::to_owned))
                .collect();
            for endpoint in [
                input["source_host_id"].as_str(),
                input["destination_host_id"].as_str(),
            ]
            .into_iter()
            .flatten()
            {
                if let Some(host) = hosts.iter().find(|h| h["id"] == endpoint) {
                    if node_ids.insert(endpoint.to_owned()) {
                        nodes.push(host.clone());
                    }
                }
            }
            for hop in selected["hops"].as_array().into_iter().flatten() {
                let id = hop["node_id"].as_str().unwrap_or("");
                if node_ids.insert(id.to_owned()) {
                    if let Some(node) = hosts
                        .iter()
                        .chain(d["devices"].as_array().unwrap())
                        .find(|n| n["id"] == id)
                    {
                        nodes.push(node.clone());
                    }
                }
            }
            for link_id in selected["links"]
                .as_array()
                .into_iter()
                .flatten()
                .filter_map(Value::as_str)
            {
                if !projected_links.iter().any(|l| l["id"] == link_id) {
                    if let Some(link) = links.iter().find(|l| l["id"] == link_id) {
                        projected_links.push(link.clone());
                    }
                }
            }
        }
    }
    if nodes.len() > 140 || projected_links.len() > LINK_LIMIT {
        return Err("projection_limit".into());
    }
    let mut canvas_width = 780usize;
    let mut canvas_height = 180usize;
    if view == "l2" {
        let target_id = nodes
            .iter()
            .find(|n| n["role"] == "switch")
            .and_then(|n| n["network_id"].as_str())
            .unwrap_or("")
            .to_owned();
        let row_h = 104usize;
        let mut host_order: Vec<usize> = nodes
            .iter()
            .enumerate()
            .filter(|(_, n)| n["role"] == "host")
            .map(|(i, _)| i)
            .collect();
        host_order.sort_by_key(|i| {
            (
                nodes[*i]["attachment"].as_str().unwrap_or("").to_owned(),
                nodes[*i]["id"].as_str().unwrap_or("").to_owned(),
            )
        });
        let mut groups: std::collections::BTreeMap<String, Vec<usize>> =
            std::collections::BTreeMap::new();
        for node in nodes.iter().filter(|n| {
            n["role"] == "switch"
                && n["network_id"] == target_id
                && !n["id"].as_str().unwrap_or("").ends_with("-d")
        }) {
            groups
                .entry(node["id"].as_str().unwrap_or("").to_owned())
                .or_default();
        }
        for index in host_order {
            groups
                .entry(nodes[index]["attachment"].as_str().unwrap_or("").to_owned())
                .or_default()
                .push(index);
        }
        // Compact attachment groups avoid a 100-row canvas. Keep card-sized
        // spacing in both axes and an empty row between adjacent fan-outs.
        let columns = groups
            .values()
            .map(|items| items.len().min(5))
            .max()
            .unwrap_or(1)
            .max(1);
        let access_x = columns * 188 + 100;
        let mut attachment_y = HashMap::new();
        let mut next_row = 0usize;
        for (attachment, indices) in groups {
            let rows = indices.len().div_ceil(columns).max(1);
            attachment_y.insert(attachment, (next_row * row_h) + ((rows - 1) * row_h) / 2);
            for (local, index) in indices.into_iter().enumerate() {
                nodes[index]["layout"] = json!({"x":(local % columns) * 188,"y":(next_row + local / columns) * row_h,"width":168,"height":84});
            }
            next_row += rows + 1;
        }
        let rows = next_row.max(1);
        for node in nodes
            .iter_mut()
            .filter(|n| n["role"] == "switch" && n["network_id"] == target_id)
        {
            let id = node["id"].as_str().unwrap_or("");
            let y = if id.ends_with("-d") {
                rows / 2 * row_h
            } else {
                attachment_y.get(id).copied().unwrap_or(0)
            };
            let x = if id.ends_with("-d") {
                access_x + 260
            } else {
                access_x
            };
            node["layout"] = json!({"x":x,"y":y,"width":168,"height":84});
        }
        for node in nodes.iter_mut().filter(|n| n["role"] == "router") {
            node["layout"] = json!({"x":access_x + 520,"y":(rows.saturating_sub(1) * row_h) / 2,"width":168,"height":84});
        }
        canvas_width = access_x + 520 + 168;
        canvas_height = rows * row_h + 84;
    }
    for link in &mut projected_links {
        let endpoint = |id: &str| {
            nodes
                .iter()
                .chain(hosts.iter())
                .chain(d["devices"].as_array().unwrap().iter())
                .find(|n| n["id"] == id)
        };
        let from = endpoint(link["from"].as_str().unwrap_or(""));
        let to = endpoint(link["to"].as_str().unwrap_or(""));
        if let (Some(from), Some(to)) = (from, to) {
            if from["vrf"].as_str().is_some() && from["vrf"] == to["vrf"] {
                link["vrf"] = from["vrf"].clone();
            }
            if from["vdom"].as_str().is_some() && from["vdom"] == to["vdom"] {
                link["vdom"] = from["vdom"].clone();
            }
        }
    }
    let filters = json!({"network_id":network,"zone":zone,"search":search,"vrf":vrf,"vdom":vdom});
    let matched_zones: HashSet<&str> = if view == "zone" {
        nodes
            .iter()
            .filter(|n| n["role"] == "zone")
            .filter_map(|n| n["zone"].as_str())
            .collect()
    } else {
        matching.iter().filter_map(|h| h["zone"].as_str()).collect()
    };
    let mut result = json!({"schema_version":1,"synthetic":true,"view":view,"layout":{"width":canvas_width,"height":canvas_height},"facets":{"vrfs":vrfs,"vdoms":vdoms,"networks":network_facets,"zones":zone_facets},"filters":filters,"counts":{"hosts":hosts.len(),"networks":d["networks"].as_array().unwrap().len(),"zones":d["zones"].as_array().unwrap().len(),"matched_networks":selected_network_ids.len(),"matched_zones":matched_zones.len(),"visible_nodes":nodes.len(),"visible_links":projected_links.len(),"matched_hosts":total},"nodes":nodes,"links":projected_links,"hosts":page,"pagination":{"offset":offset,"limit":limit,"total":total,"has_more":offset.saturating_add(limit)<total},"navigation":{"focus_host_id":focus_host_id,"focus_status":focus_status,"page_index":page_index,"page_count":page_count,"first_offset":0,"last_offset":last_offset,"previous_offset":if offset > 0 { Some(offset.saturating_sub(limit)) } else { None },"next_offset":if offset.saturating_add(limit)<total { Some(offset+limit) } else { None }},"sources":sources});
    if let Some(path) = path_result {
        result["path"] = path;
    }
    if view == "l2" && result["path"]["status"] == "modeled_l2_path" {
        let hops = result["path"]["hops"].as_array().ok_or("invalid_path")?;
        let horizontal_nodes: Vec<Value> = hops
            .iter()
            .enumerate()
            .map(|(i, hop)| json!({"id":hop["node_id"],"layout":{"x":i*220,"y":0,"width":168,"height":84}}))
            .collect();
        let vertical_nodes: Vec<Value> = hops
            .iter()
            .enumerate()
            .map(|(i, hop)| json!({"id":hop["node_id"],"layout":{"x":0,"y":i*120,"width":168,"height":84}}))
            .collect();
        let horizontal_width = hops.len().saturating_sub(1) * 220 + 168;
        let vertical_height = hops.len().saturating_sub(1) * 120 + 84;
        result["path_layout"] = json!({
            "horizontal":{"width":horizontal_width,"height":84,"nodes":horizontal_nodes},
            "vertical":{"width":168,"height":vertical_height,"nodes":vertical_nodes}
        });
    }
    Ok(result)
}

fn path(hosts: &[Value], links: &[Value], src: &str, dst: &str) -> Value {
    let a = hosts.iter().find(|h| h["id"] == src);
    let b = hosts.iter().find(|h| h["id"] == dst);
    let (Some(a), Some(b)) = (a, b) else {
        return json!({"status":"indeterminate","reason":"host_not_found","hops":[],"links":[],"evidence":[]});
    };
    if a["network_id"] != b["network_id"] || a["vlan"] != b["vlan"] {
        return json!({"status":"requires_l3","reason":"different_network_or_vlan","hops":[],"links":[],"evidence":[]});
    }
    let mut adj: HashMap<String, Vec<(String, &Value)>> = HashMap::new();
    let host_ids: HashSet<String> = hosts
        .iter()
        .filter_map(|h| h["id"].as_str().map(str::to_owned))
        .collect();
    for l in links.iter().filter(|l| {
        matches!(l["kind"].as_str(), Some("attachment" | "trunk")) && l["vlan"] == a["vlan"]
    }) {
        let x = l["from"].as_str().unwrap_or("").to_owned();
        let y = l["to"].as_str().unwrap_or("").to_owned();
        adj.entry(x.clone()).or_default().push((y.clone(), l));
        adj.entry(y).or_default().push((x, l));
    }
    let mut queue = VecDeque::from([src.to_owned()]);
    let mut prev: HashMap<String, (String, String)> = HashMap::new();
    let mut seen = HashSet::from([src.to_owned()]);
    while let Some(cur) = queue.pop_front() {
        if cur == dst {
            break;
        }
        if cur != src && host_ids.contains(&cur) {
            continue;
        }
        for (next, l) in adj.get(&cur).into_iter().flatten() {
            if seen.insert(next.clone()) {
                prev.insert(
                    next.clone(),
                    (cur.clone(), l["id"].as_str().unwrap_or("").into()),
                );
                queue.push_back(next.clone());
            }
        }
    }
    if !seen.contains(dst) {
        return json!({"status":"indeterminate","reason":"no_explicit_same_vlan_path","hops":[],"links":[],"evidence":[]});
    }
    let mut ids = Vec::new();
    let mut at = dst.to_owned();
    while at != src {
        let Some((p, l)) = prev.get(&at).cloned() else {
            return json!({"status":"indeterminate","reason":"no_explicit_same_vlan_path","hops":[],"links":[],"evidence":[]});
        };
        ids.push(l);
        at = p;
        if ids.len() > 32 {
            return json!({"status":"indeterminate","reason":"modeled_path_exceeds_limit","hops":[],"links":[],"evidence":[]});
        }
    }
    ids.reverse();
    let mut chain = vec![src.to_owned()];
    let mut current = src.to_owned();
    for id in &ids {
        let l = links
            .iter()
            .find(|l| l["id"].as_str() == Some(id.as_str()))
            .unwrap();
        let next = if l["from"].as_str() == Some(current.as_str()) {
            l["to"].as_str().unwrap_or("")
        } else {
            l["from"].as_str().unwrap_or("")
        };
        chain.push(next.to_owned());
        current = next.into();
    }
    let evidence: Vec<Value> = ids
        .iter()
        .filter_map(|id| links.iter().find(|l| l["id"].as_str() == Some(id.as_str())))
        .flat_map(|l| l["evidence"].as_array().cloned().unwrap_or_default())
        .collect();
    let hops: Vec<Value> = chain
        .iter()
        .enumerate()
        .map(|(i, id)| {
            let ingress = if i == 0 {
                Value::Null
            } else {
                let l = links
                    .iter()
                    .find(|l| l["id"].as_str() == Some(ids[i - 1].as_str()))
                    .unwrap();
                if l["from"].as_str() == Some(id.as_str()) {
                    l["from_port"].clone()
                } else {
                    l["to_port"].clone()
                }
            };
            let egress = if i + 1 == chain.len() {
                Value::Null
            } else {
                let l = links
                    .iter()
                    .find(|l| l["id"].as_str() == Some(ids[i].as_str()))
                    .unwrap();
                if l["from"].as_str() == Some(id.as_str()) {
                    l["from_port"].clone()
                } else {
                    l["to_port"].clone()
                }
            };
            let own = hosts
                .iter()
                .find(|h| h["id"].as_str() == Some(id.as_str()))
                .map(|h| h["evidence"].clone())
                .unwrap_or_else(|| json!([]));
            let mut refs = own.as_array().cloned().unwrap_or_default();
            for index in [i.checked_sub(1), if i < ids.len() { Some(i) } else { None }]
                .into_iter()
                .flatten()
            {
                let l = links
                    .iter()
                    .find(|l| l["id"].as_str() == Some(ids[index].as_str()))
                    .unwrap();
                refs.extend(l["evidence"].as_array().cloned().unwrap_or_default())
            }
            let mut hop = json!({"node_id":id,"ingress":ingress,"egress":egress,"vlan":a["vlan"],"evidence":refs});
            if let Some(vrf) = a["vrf"].as_str() { hop["vrf"] = json!(vrf); }
            if let Some(vdom) = a["vdom"].as_str() { hop["vdom"] = json!(vdom); }
            hop
        })
        .collect();
    json!({"status":"modeled_l2_path","reason":"explicit_same_vlan_links","hops":hops,"links":ids,"evidence":evidence})
}

#[cfg(test)]
mod tests {
    use super::*;
    fn replace_source_text(source_row: &mut Value, text: String) {
        source_row["sha256"] = json!(hash(&text));
        source_row["text"] = json!(text);
        source_row["lines"] = json!(source_row["text"]
            .as_str()
            .unwrap()
            .lines()
            .enumerate()
            .map(|(i, line)| json!({"number":i+1,"text":line}))
            .collect::<Vec<_>>());
    }
    fn replace_vrf_context(d: &mut Value, old: &str, new: &str) {
        for key in ["hosts", "devices", "networks", "links"] {
            for row in d[key].as_array_mut().unwrap() {
                if row["vrf"] == old {
                    row["vrf"] = json!(new);
                }
            }
        }
        for source in d["sources"].as_array_mut().unwrap() {
            if source["id"] != "synthetic-records" {
                let text = source["text"].as_str().unwrap().replace(old, new);
                replace_source_text(source, text);
            }
        }
        let canonical = d["hosts"]
            .as_array()
            .unwrap()
            .iter()
            .chain(d["devices"].as_array().unwrap())
            .chain(d["networks"].as_array().unwrap())
            .chain(d["links"].as_array().unwrap())
            .map(|row| canonical_record(row).unwrap())
            .collect::<Vec<_>>()
            .join("\n");
        let source = d["sources"]
            .as_array_mut()
            .unwrap()
            .iter_mut()
            .find(|s| s["id"] == "synthetic-records")
            .unwrap();
        replace_source_text(source, canonical);
        let digests: HashMap<String, String> = d["sources"]
            .as_array()
            .unwrap()
            .iter()
            .map(|s| {
                (
                    s["id"].as_str().unwrap().to_owned(),
                    s["sha256"].as_str().unwrap().to_owned(),
                )
            })
            .collect();
        for key in ["hosts", "devices", "networks", "links"] {
            for row in d[key].as_array_mut().unwrap() {
                for ev in row["evidence"].as_array_mut().unwrap() {
                    let sid = ev["source_id"].as_str().unwrap().to_owned();
                    let digest = &digests[&sid];
                    ev["sha256"] = json!(digest);
                    ev["ref"] = json!(format!(
                        "{sid}:{}:L{}",
                        &digest[..12],
                        ev["line"].as_u64().unwrap()
                    ));
                }
            }
        }
    }
    fn legacy_dataset() -> Value {
        let mut d = layer_sample(json!({})).unwrap()["dataset"].clone();
        for key in ["hosts", "devices", "networks"] {
            for row in d[key].as_array_mut().unwrap() {
                if let Some(obj) = row.as_object_mut() {
                    obj.remove("vrf");
                    obj.remove("vdom");
                }
            }
        }
        for source_row in d["sources"].as_array_mut().unwrap() {
            let id = source_row["id"].as_str().unwrap_or("");
            if id.starts_with("inventory-") {
                let text = source_row["text"]
                    .as_str()
                    .unwrap()
                    .lines()
                    .map(|line| line.split_once(" vrf ").map_or(line, |(prefix, _)| prefix))
                    .collect::<Vec<_>>()
                    .join("\n");
                replace_source_text(source_row, text);
            }
        }
        let canonical = d["hosts"]
            .as_array()
            .unwrap()
            .iter()
            .chain(d["devices"].as_array().unwrap())
            .chain(d["networks"].as_array().unwrap())
            .chain(d["links"].as_array().unwrap())
            .map(|row| canonical_record(row).unwrap())
            .collect::<Vec<_>>()
            .join("\n");
        let canon_source = d["sources"]
            .as_array_mut()
            .unwrap()
            .iter_mut()
            .find(|s| s["id"] == "synthetic-records")
            .unwrap();
        replace_source_text(canon_source, canonical);
        let digests: HashMap<String, String> = d["sources"]
            .as_array()
            .unwrap()
            .iter()
            .map(|s| {
                (
                    s["id"].as_str().unwrap().to_owned(),
                    s["sha256"].as_str().unwrap().to_owned(),
                )
            })
            .collect();
        for key in ["hosts", "devices", "networks", "links"] {
            for row in d[key].as_array_mut().unwrap() {
                for ev in row["evidence"].as_array_mut().unwrap() {
                    let sid = ev["source_id"].as_str().unwrap().to_owned();
                    let digest = &digests[&sid];
                    ev["sha256"] = json!(digest);
                    ev["ref"] = json!(format!(
                        "{sid}:{}:L{}",
                        &digest[..12],
                        ev["line"].as_u64().unwrap()
                    ));
                }
            }
        }
        d
    }
    #[test]
    fn sample_is_deterministic_and_exactly_one_thousand_hosts() {
        let a = layer_sample(json!({})).unwrap()["dataset"].clone();
        let b = layer_sample(json!({})).unwrap()["dataset"].clone();
        assert_eq!(a, b);
        assert_eq!(a["hosts"].as_array().unwrap().len(), 1000);
        assert_eq!(a["hosts"][999]["id"], "host-1000");
    }
    #[test]
    fn large_synthetic_projection_scale_respects_source_budget() {
        let rejected_started = std::time::Instant::now();
        assert_eq!(
            layer_sample_sized(40, 250).unwrap_err(),
            "source_limit",
            "10,000-host evidence corpus must stay within the existing source budget"
        );
        let rejected_10k_ms = rejected_started.elapsed().as_secs_f64() * 1000.0;

        let build_started = std::time::Instant::now();
        let d = layer_sample_sized(10, 250).expect("large synthetic corpus")["dataset"].clone();
        let build_ms = build_started.elapsed().as_secs_f64() * 1000.0;
        assert_eq!(d["hosts"].as_array().unwrap().len(), 2_500);
        let dataset_bytes = serde_json::to_vec(&d).unwrap().len();

        let projection_started = std::time::Instant::now();
        let out =
            project_layers(json!({"dataset":d,"view":"l2","network_id":"net-10","limit":100}))
                .expect("validated large projection");
        let projection_ms = projection_started.elapsed().as_secs_f64() * 1000.0;
        let response_bytes = serde_json::to_vec(&out).unwrap().len();
        assert_eq!(out["counts"]["hosts"], 2_500);
        assert_eq!(out["pagination"]["total"], 250);
        assert!(out["nodes"].as_array().unwrap().len() <= 140);
        eprintln!(
            "scale_projection view=l2 hosts=2500 dataset_bytes={dataset_bytes} response_bytes={response_bytes} build_ms={build_ms:.2} projection_ms={projection_ms:.2} rejected_hosts=10000 rejected_reason=source_limit reject_ms={rejected_10k_ms:.2}"
        );
    }
    #[test]
    fn projections_page_and_paths_are_bounded_and_semantic() {
        let d = layer_sample(json!({})).unwrap()["dataset"].clone();
        let o=project_layers(json!({"dataset":d,"view":"l2","network_id":"net-01","limit":100,"source_host_id":"host-0001","destination_host_id":"host-0051"})).unwrap();
        assert_eq!(o["pagination"]["total"], 100);
        assert_eq!(o["path"]["status"], "modeled_l2_path");
        assert_eq!(o["counts"]["hosts"], 1000);
        let d = layer_sample(json!({})).unwrap()["dataset"].clone();
        let o=project_layers(json!({"dataset":d,"view":"l2","source_host_id":"host-0001","destination_host_id":"host-0101"})).unwrap();
        assert_eq!(o["path"]["status"], "requires_l3");
    }
    #[test]
    fn rejects_invalid_view_and_duplicate_hosts() {
        let d = layer_sample(json!({})).unwrap()["dataset"].clone();
        assert!(project_layers(json!({"dataset":d.clone(),"view":"bad"})).is_err());
        let mut bad = d;
        let duplicate = bad["hosts"][0].clone();
        bad["hosts"][1] = duplicate;
        assert_eq!(validate_dataset(&bad).unwrap_err(), "duplicate_host");
    }
    #[test]
    fn source_hash_and_host_semantics_are_bound_to_evidence() {
        let d = layer_sample(json!({})).unwrap()["dataset"].clone();
        let mut bad = d.clone();
        bad["sources"][2]["text"] = "modified source".into();
        assert_eq!(validate_dataset(&bad).unwrap_err(), "invalid_source");
        let mut bad = d;
        bad["hosts"][0]["vlan"] = json!(999);
        assert_eq!(
            validate_dataset(&bad).unwrap_err(),
            "record_evidence_mismatch"
        );
    }
    #[test]
    fn rejects_dangling_links_and_invalid_requests() {
        let d = layer_sample(json!({})).unwrap()["dataset"].clone();
        let mut bad = d.clone();
        bad["links"][0]["to"] = "missing-switch".into();
        assert_eq!(
            validate_dataset(&bad).unwrap_err(),
            "record_evidence_mismatch"
        );
        assert!(project_layers(json!({"dataset":d,"view":"l2","limit":"100"})).is_err());
        assert!(project_layers(json!({"dataset":null,"view":"l2","unknown":true})).is_err());
    }
    #[test]
    fn hops_report_real_ports_and_destination_evidence() {
        let d = layer_sample(json!({})).unwrap()["dataset"].clone();
        let out=project_layers(json!({"dataset":d,"view":"l2","limit":25,"source_host_id":"host-0001","destination_host_id":"host-0075"})).unwrap();
        let path = &out["path"];
        assert_eq!(path["status"], "modeled_l2_path");
        assert_eq!(path["hops"][0]["vrf"], "lab-vrf-01");
        assert_eq!(path["hops"][0]["vdom"], "campus");
        assert_eq!(path["hops"][0]["egress"], "eth0");
        assert!(path["hops"][1]["egress"]
            .as_str()
            .unwrap()
            .starts_with("Gi1/0/"));
        assert_eq!(
            path["hops"].as_array().unwrap().last().unwrap()["evidence"][0]["source_id"],
            "inventory-01"
        );
        assert!(out["nodes"]
            .as_array()
            .unwrap()
            .iter()
            .any(|n| n["id"] == "host-0075"));
        assert!(out["links"]
            .as_array()
            .unwrap()
            .iter()
            .filter(|l| l["kind"] == "attachment")
            .all(|l| l["vrf"] == "lab-vrf-01" && l["vdom"] == "campus"));
    }

    #[test]
    fn modeled_path_layouts_follow_hops_without_changing_path_or_sources() {
        let d = layer_sample(json!({})).unwrap()["dataset"].clone();
        let original_sources = d["sources"].clone();
        let out = project_layers(json!({"dataset":d.clone(),"view":"l2","source_host_id":"host-0001","destination_host_id":"host-0075"})).unwrap();
        let hops = out["path"]["hops"].as_array().unwrap();
        let expected_ids: Vec<&str> = hops
            .iter()
            .map(|hop| hop["node_id"].as_str().unwrap())
            .collect();
        for (orientation, dx, dy) in [("horizontal", 220u64, 0u64), ("vertical", 0u64, 120u64)] {
            let canvas = &out["path_layout"][orientation];
            let layout_nodes = canvas["nodes"].as_array().unwrap();
            let ids: Vec<&str> = layout_nodes
                .iter()
                .map(|node| node["id"].as_str().unwrap())
                .collect();
            assert_eq!(ids, expected_ids);
            let mut previous_end = 0;
            for (i, node) in layout_nodes.iter().enumerate() {
                let card = &node["layout"];
                let x = card["x"].as_u64().unwrap();
                let y = card["y"].as_u64().unwrap();
                assert_eq!(card["width"], 168);
                assert_eq!(card["height"], 84);
                assert_eq!(x, i as u64 * dx);
                assert_eq!(y, i as u64 * dy);
                if orientation == "horizontal" {
                    assert!(x >= previous_end);
                    previous_end = x + 168;
                } else {
                    assert!(y >= previous_end);
                    previous_end = y + 84;
                }
            }
            if orientation == "horizontal" {
                assert_eq!(canvas["width"], (hops.len() - 1) as u64 * 220 + 168);
                assert_eq!(canvas["height"], 84);
            } else {
                assert_eq!(canvas["width"], 168);
                assert_eq!(canvas["height"], (hops.len() - 1) as u64 * 120 + 84);
            }
        }
        assert_eq!(out["sources"], original_sources);
        assert!(out["path"].get("layout").is_none());
        assert!(hops.iter().all(|hop| hop.get("layout").is_none()));
    }

    #[test]
    fn scope_filters_are_intersected_and_paths_fail_closed_outside_context() {
        let d = layer_sample(json!({})).unwrap()["dataset"].clone();
        let out = project_layers(json!({"dataset":d,"view":"l2","vrf":" lab-vrf-01 ","vdom":"campus","network_id":"net-01","zone":"engineering","search":"host-00","limit":100})).unwrap();
        assert_eq!(out["pagination"]["total"], 99);
        assert_eq!(out["facets"]["vrfs"].as_array().unwrap().len(), 10);
        assert_eq!(out["facets"]["vdoms"].as_array().unwrap().len(), 2);
        assert_eq!(out["filters"]["vrf"], "lab-vrf-01");
        let distinct_case =
            project_layers(json!({"dataset":d,"view":"l2","vrf":"LAB-VRF-01"})).unwrap();
        assert_eq!(distinct_case["pagination"]["total"], 0);
        assert_eq!(distinct_case["filters"]["vrf"], "LAB-VRF-01");
        let routed = project_layers(json!({"dataset":d,"view":"l2","network_id":"net-01","search":"host-0001","source_host_id":"host-0001","destination_host_id":"host-0101"})).unwrap();
        assert_eq!(routed["path"]["status"], "requires_l3");
        let same_vlan = project_layers(json!({"dataset":d,"view":"l2","vrf":"lab-vrf-01","search":"host-0001","source_host_id":"host-0001","destination_host_id":"host-0002"})).unwrap();
        assert_eq!(same_vlan["path"]["status"], "modeled_l2_path");
        let outside = project_layers(json!({"dataset":d,"view":"l2","vrf":"lab-vrf-02","source_host_id":"host-0001","destination_host_id":"host-0002"})).unwrap();
        assert_eq!(outside["path"]["reason"], "outside_selected_context");
        assert_eq!(outside["path"]["hops"].as_array().unwrap().len(), 0);
        assert!(!outside["nodes"]
            .as_array()
            .unwrap()
            .iter()
            .any(|n| n["id"] == "host-0001"));
    }

    #[test]
    fn semantic_search_and_pagination_navigation_are_scoped() {
        let d = layer_sample(json!({})).unwrap()["dataset"].clone();
        let out = project_layers(
            json!({"dataset":d.clone(),"view":"l2","limit":25,"focus_host_id":"host-0999"}),
        )
        .unwrap();
        assert_eq!(out["pagination"]["offset"], 975);
        assert_eq!(out["navigation"]["focus_status"], "visible");
        assert_eq!(out["navigation"]["page_index"], 40);
        assert_eq!(out["navigation"]["page_count"], 40);
        assert_eq!(out["navigation"]["previous_offset"], 950);
        assert_eq!(out["navigation"]["next_offset"], Value::Null);
        let out = project_layers(
            json!({"dataset":d.clone(),"view":"l2","limit":100,"focus_host_id":"host-0999"}),
        )
        .unwrap();
        assert_eq!(out["pagination"]["offset"], 900);
        assert_eq!(out["hosts"][0]["id"], "host-0901");
        let focused_over_source = project_layers(json!({"dataset":d.clone(),"view":"l2","limit":25,"focus_host_id":"host-0999","source_host_id":"host-0001"})).unwrap();
        assert_eq!(focused_over_source["navigation"]["focus_status"], "visible");
        assert!(focused_over_source["nodes"]
            .as_array()
            .unwrap()
            .iter()
            .any(|n| n["id"] == "host-0999"));
        let filtered = project_layers(json!({"dataset":d.clone(),"view":"l2","limit":25,"focus_host_id":"host-0999","zone":"engineering"})).unwrap();
        assert_eq!(filtered["navigation"]["focus_status"], "outside_filters");
        assert!(filtered["hosts"]
            .as_array()
            .unwrap()
            .iter()
            .all(|h| h["zone"] == "engineering"));
        let absent =
            project_layers(json!({"dataset":d.clone(),"view":"l2","focus_host_id":"missing-host"}))
                .unwrap();
        assert_eq!(absent["navigation"]["focus_status"], "not_found");
        let escaped = project_layers(json!({"dataset":d,"view":"l2","focus_host_id":"host-0999","vrf":"lab-vrf-01","source_host_id":"host-0001","destination_host_id":"host-0999"})).unwrap();
        assert_eq!(escaped["navigation"]["focus_status"], "outside_filters");
        assert_eq!(escaped["path"]["reason"], "outside_selected_context");
        assert!(escaped["nodes"]
            .as_array()
            .unwrap()
            .iter()
            .all(|n| n["id"] != "host-0999"));
    }

    #[test]
    fn stale_offsets_clamp_and_search_excludes_evidence_blobs() {
        let d = layer_sample(json!({})).unwrap()["dataset"].clone();
        let arbitrary =
            project_layers(json!({"dataset":d.clone(),"view":"l2","limit":25,"offset":7})).unwrap();
        assert_eq!(arbitrary["pagination"]["offset"], 7);
        assert_eq!(arbitrary["hosts"][0]["id"], "host-0008");
        assert_eq!(arbitrary["navigation"]["previous_offset"], 0);
        let last_valid =
            project_layers(json!({"dataset":d.clone(),"view":"l2","limit":25,"offset":980}))
                .unwrap();
        assert_eq!(last_valid["pagination"]["offset"], 980);
        assert_eq!(last_valid["hosts"][0]["id"], "host-0981");
        let stale =
            project_layers(json!({"dataset":d.clone(),"view":"l2","limit":25,"offset":9999}))
                .unwrap();
        assert_eq!(stale["pagination"]["offset"], 975);
        assert_eq!(stale["hosts"].as_array().unwrap().len(), 25);
        let at_end =
            project_layers(json!({"dataset":d.clone(),"view":"l2","limit":25,"offset":1000}))
                .unwrap();
        assert_eq!(at_end["pagination"]["offset"], 975);
        let hash = d["hosts"][0]["evidence"][0]["sha256"]
            .as_str()
            .unwrap()
            .to_owned();
        let evidence_ref = d["hosts"][0]["evidence"][0]["ref"]
            .as_str()
            .unwrap()
            .to_owned();
        let by_hash =
            project_layers(json!({"dataset":d.clone(),"view":"l2","search":hash})).unwrap();
        assert_eq!(by_hash["pagination"]["total"], 0);
        let by_evidence_ref =
            project_layers(json!({"dataset":d,"view":"l2","search":evidence_ref})).unwrap();
        assert_eq!(by_evidence_ref["pagination"]["total"], 0);
    }

    #[test]
    fn network_and_zone_facets_intersect_other_filters() {
        let d = layer_sample(json!({})).unwrap()["dataset"].clone();
        let out = project_layers(
            json!({"dataset":d,"view":"l2","zone":"engineering","vrf":"lab-vrf-01","limit":25}),
        )
        .unwrap();
        let networks = out["facets"]["networks"].as_array().unwrap();
        assert_eq!(networks.len(), 1);
        assert_eq!(
            networks[0],
            json!({"value":"net-01","label":"net-01","count":100})
        );
        let zones = out["facets"]["zones"].as_array().unwrap();
        assert_eq!(zones.len(), 1);
        assert_eq!(zones[0]["value"], "engineering");
        assert_eq!(zones[0]["count"], 100);
        assert_eq!(out["facets"]["vrfs"].as_array().unwrap().len(), 10);
    }

    #[test]
    fn declared_mixed_case_context_matches_only_exactly() {
        let mut d = layer_sample(json!({})).unwrap()["dataset"].clone();
        replace_vrf_context(&mut d, "lab-vrf-01", "Lab-VRF-01");
        assert_eq!(validate_dataset(&d).unwrap(), ());
        let exact =
            project_layers(json!({"dataset":d.clone(),"view":"l2","vrf":"Lab-VRF-01"})).unwrap();
        assert_eq!(exact["pagination"]["total"], 100);
        let wrong_case =
            project_layers(json!({"dataset":d,"view":"l2","vrf":"lab-vrf-01"})).unwrap();
        assert_eq!(wrong_case["pagination"]["total"], 0);
    }

    #[test]
    fn l2_fanout_layout_cards_do_not_overlap() {
        let d = layer_sample(json!({})).unwrap()["dataset"].clone();
        let out =
            project_layers(json!({"dataset":d,"view":"l2","network_id":"net-01","limit":100}))
                .unwrap();
        let nodes = out["nodes"].as_array().unwrap();
        let mut cards = Vec::new();
        for node in nodes {
            let l = &node["layout"];
            let x = l["x"].as_u64().unwrap();
            let y = l["y"].as_u64().unwrap();
            let w = l["width"].as_u64().unwrap();
            let h = l["height"].as_u64().unwrap();
            for (ox, oy, ow, oh) in &cards {
                assert!(
                    x + w <= *ox || *ox + *ow <= x || y + h <= *oy || *oy + *oh <= y,
                    "card overlap"
                );
            }
            cards.push((x, y, w, h));
        }
        assert_eq!(cards.len(), 104);
        let width = out["layout"]["width"].as_u64().unwrap();
        let height = out["layout"]["height"].as_u64().unwrap();
        assert!(
            width <= 2_200 && height <= 3_000,
            "fan-out must fit a compact overview"
        );
        assert!(cards
            .iter()
            .all(|(x, y, w, h)| x + w <= width && y + h <= height));
        let a_bottom = nodes
            .iter()
            .filter(|n| n["attachment"] == "sw-01-a")
            .map(|n| n["layout"]["y"].as_u64().unwrap() + n["layout"]["height"].as_u64().unwrap())
            .max()
            .unwrap();
        let b_top = nodes
            .iter()
            .filter(|n| n["attachment"] == "sw-01-b")
            .map(|n| n["layout"]["y"].as_u64().unwrap())
            .min()
            .unwrap();
        assert!(
            a_bottom < b_top,
            "attachment fan-outs must remain visibly separate"
        );
        assert!(
            nodes.iter().find(|n| n["id"] == "sw-01-a").unwrap()["layout"]["y"]
                .as_u64()
                .unwrap()
                < nodes.iter().find(|n| n["id"] == "sw-01-b").unwrap()["layout"]["y"]
                    .as_u64()
                    .unwrap()
        );
    }

    #[test]
    fn scope_filters_bound_l2_l3_and_zone_projection_context() {
        let d = layer_sample(json!({})).unwrap()["dataset"].clone();
        let incompatible = project_layers(
            json!({"dataset":d,"view":"l2","network_id":"net-01","vrf":"lab-vrf-02","limit":100}),
        )
        .unwrap();
        assert_eq!(incompatible["counts"]["visible_nodes"], 0);
        assert_eq!(incompatible["links"].as_array().unwrap().len(), 0);
        let d = layer_sample(json!({})).unwrap()["dataset"].clone();
        let l3 = project_layers(json!({"dataset":d,"view":"l3","network_id":"net-01","vrf":"lab-vrf-01","vdom":"campus"})).unwrap();
        let net = l3["nodes"]
            .as_array()
            .unwrap()
            .iter()
            .find(|n| n["role"] == "network")
            .unwrap();
        assert_eq!(net["vrf"], "lab-vrf-01");
        assert_eq!(net["vdom"], "campus");
        let d = layer_sample(json!({})).unwrap()["dataset"].clone();
        let zone =
            project_layers(json!({"dataset":d,"view":"zone","vrf":"lab-vrf-01","vdom":"campus"}))
                .unwrap();
        assert!(zone["nodes"]
            .as_array()
            .unwrap()
            .iter()
            .all(|n| n["zone"] == "engineering" || n["zone"] == "shared-services"));
        assert!(!zone["nodes"]
            .as_array()
            .unwrap()
            .iter()
            .flat_map(|n| n["evidence"].as_array().unwrap().iter())
            .any(|e| e["source_id"] == "inventory-02"));
        let d = layer_sample(json!({})).unwrap()["dataset"].clone();
        let none =
            project_layers(json!({"dataset":d,"view":"zone","vrf":"lab-vrf-01","vdom":"root"}))
                .unwrap();
        assert_eq!(none["nodes"].as_array().unwrap().len(), 0);
        assert_eq!(none["links"].as_array().unwrap().len(), 0);
    }

    #[test]
    fn rejects_scope_metadata_that_disagrees_with_canonical_evidence() {
        let mut d = layer_sample(json!({})).unwrap()["dataset"].clone();
        d["hosts"][0]["vrf"] = "lab-vrf-02".into();
        assert!(matches!(
            validate_dataset(&d).unwrap_err().as_str(),
            "invalid_host_network" | "record_evidence_mismatch"
        ));
        let mut malformed = layer_sample(json!({})).unwrap()["dataset"].clone();
        malformed["hosts"][0]["vrf"] = "invalid scope".into();
        assert_eq!(validate_dataset(&malformed).unwrap_err(), "invalid_scope");
        let d = layer_sample(json!({})).unwrap()["dataset"].clone();
        assert_eq!(
            project_layers(json!({"dataset":d,"view":"l2","vrf":"x".repeat(81)})).unwrap_err(),
            "invalid_layer_request"
        );
    }

    #[test]
    fn accepts_legacy_schema_one_dataset_without_scope_metadata() {
        let d = legacy_dataset();
        validate_dataset(&d).unwrap();
        let out = project_layers(json!({"dataset":d,"view":"l2","vrf":"lab-vrf-01"})).unwrap();
        assert_eq!(out["pagination"]["total"], 0);
        assert_eq!(out["nodes"].as_array().unwrap().len(), 0);
        assert!(out["facets"]["vrfs"].as_array().unwrap().is_empty());
    }
}
