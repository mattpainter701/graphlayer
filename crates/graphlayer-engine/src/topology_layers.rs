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
    let mut hosts = Vec::with_capacity(1000);
    let mut devices = Vec::with_capacity(42);
    let mut links = Vec::with_capacity(1060);
    let mut networks = Vec::new();
    let zones = ["engineering", "operations", "research", "shared-services"];
    for n in 1..=10 {
        let subnet = format!("10.20.{}.0/24", n);
        let net_id = format!("net-{n:02}");
        let zone = zones[(n - 1) % zones.len()];
        let inv_id = format!("inventory-{n:02}");
        let mut lines = vec![format!(
            "network {net_id} {subnet} zone {zone} gateway 10.20.{n}.253 via rtr-{n:02}"
        )];
        for h in 1..=100 {
            let mac = format!(
                "02:20:{n:02x}:{:02x}:{:02x}:{:02x}",
                (h >> 8) & 255,
                h & 255,
                n
            );
            lines.push(format!(
                "host host-{:04} 10.20.{}.{} mac {} vlan {} port Gi1/0/{} arp 10.20.{}.{} {}",
                (n - 1) * 100 + h,
                n,
                h,
                mac,
                100 + n,
                if h <= 50 { h } else { h - 50 },
                n,
                h,
                mac
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
        devices.push(json!({"id":sw_a,"label":sw_a,"role":"switch","zone":zone,"network_id":net_id,"subnet":subnet,"host_count":50,"evidence":[refev(&inv_id,1,&invhash)]}));
        devices.push(json!({"id":sw_b,"label":sw_b,"role":"switch","zone":zone,"network_id":net_id,"subnet":subnet,"host_count":50,"evidence":[refev(&inv_id,1,&invhash)]}));
        devices.push(json!({"id":dist,"label":dist,"role":"switch","zone":zone,"network_id":net_id,"subnet":subnet,"host_count":0,"evidence":[refev(&inv_id,1,&invhash)]}));
        devices.push(json!({"id":format!("rtr-{n:02}"),"label":format!("Router {n:02}"),"role":"router","zone":zone,"network_id":net_id,"subnet":subnet,"ip":format!("10.20.{n}.253"),"host_count":0,"evidence":[refev(&inv_id,1,&invhash)]}));
        networks.push(
            json!({"id":net_id,"subnet":subnet,"zone":zone,"evidence":[refev(&inv_id,1,&invhash)]}),
        );
        for h in 1..=100 {
            let ix = (n - 1) * 100 + h;
            let id = format!("host-{ix:04}");
            let ip = format!("{prefix}.{h}");
            let mac = format!(
                "02:20:{n:02x}:{:02x}:{:02x}:{:02x}",
                (h >> 8) & 255,
                h & 255,
                n
            );
            let sw = if h <= 50 { &sw_a } else { &sw_b };
            let port = if h <= 50 { h } else { h - 50 };
            hosts.push(json!({"id":id,"label":id,"role":"host","zone":zone,"network_id":net_id,"subnet":subnet,"ip":ip,"mac":mac,"vlan":100+n,"port":format!("Gi1/0/{port}"),"attachment":sw,"evidence":[refev(&inv_id,h+1,&invhash)]}));
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
        links.push(json!({"id":uplink,"from":format!("rtr-{}",if net=="net-01" {"01"} else {"02"}),"to":id,"kind":"declared_uplink","label":"Declared synthetic firewall uplink","from_port":"Gi0/1","to_port":interface,"evidence":[refev(inventory_id,102,&inventory_hash),refev(source_id,config_line,digest)]}));
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
        if !only(net, &["id", "subnet", "zone", "evidence"]) {
            return Err("invalid_dataset".into());
        }
        let id = net["id"].as_str().ok_or("invalid_dataset")?;
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
                "host_count",
                "evidence",
            ],
        ) {
            return Err("invalid_dataset".into());
        }
        let id = node["id"].as_str().ok_or("invalid_dataset")?;
        if !ids.insert(id.to_owned()) {
            return Err("duplicate_node".into());
        }
        if let Some(ip) = node["ip"].as_str() {
            let parsed = ip.parse::<Ipv4Addr>().map_err(|_| "invalid_device_ip")?;
            if !parsed.is_private() || !ips.insert(ip.to_owned()) {
                return Err("invalid_device_ip".into());
            }
        }
    }
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
                "evidence",
            ],
        ) {
            return Err("invalid_host".into());
        }
        let id = h["id"].as_str().ok_or("invalid_dataset")?;
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
        if h["role"] != "host"
            || h["subnet"] != *subnet
            || h["zone"] != *zone
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
        let actual: Vec<String> = text.lines().map(str::to_owned).collect();
        if source_lines
            .insert(
                id.to_owned(),
                (
                    actual.clone(),
                    s["sha256"].as_str().unwrap_or("").to_owned(),
                    text.to_owned(),
                ),
            )
            .is_some()
            || s["sha256"].as_str() != Some(hash(text).as_str())
            || lines.len() != actual.len()
        {
            return Err("invalid_source".into());
        }
        for (i, (line, expected)) in lines.iter().zip(&actual).enumerate() {
            if !only(line, &["number", "text"])
                || line["number"].as_u64() != Some((i + 1) as u64)
                || line["text"].as_str() != Some(expected.as_str())
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
                if source_rows.get(line.saturating_sub(1)).map(String::as_str)
                    != Some(canonical.as_str())
                {
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
        let device = devices
            .iter()
            .find(|n| {
                n["id"] == switch
                    && n["role"] == "switch"
                    && n["network_id"] == network
                    && n["zone"] == h["zone"]
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
            let host = hosts
                .iter()
                .find(|h| h["id"] == l["from"])
                .ok_or("invalid_attachment")?;
            let sw = devices
                .iter()
                .find(|n| {
                    n["id"] == l["to"]
                        && n["role"] == "switch"
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
            let sw_a = devices
                .iter()
                .find(|n| n["id"] == l["from"])
                .ok_or("invalid_link")?;
            let sw_b = devices
                .iter()
                .find(|n| n["id"] == l["to"])
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
            if line != &expected {
                return Err("invalid_link".into());
            }
        }
        if l["kind"] == "routed_uplink" {
            let sw = devices
                .iter()
                .find(|n| n["id"] == l["from"])
                .ok_or("invalid_link")?;
            let router = devices
                .iter()
                .find(|n| n["id"] == l["to"])
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
            if line != &expected {
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
            let router = devices
                .iter()
                .find(|n| {
                    n["id"] == l["to"]
                        && n["role"] == "router"
                        && n["network_id"] == l["from"]
                        && n["zone"] == net["zone"]
                })
                .ok_or("invalid_link")?;
            let expected = format!(
                "network {} {} zone {} gateway {} via {}",
                net["id"].as_str().ok_or("invalid_link")?,
                net["subnet"].as_str().ok_or("invalid_link")?,
                net["zone"].as_str().ok_or("invalid_link")?,
                router["ip"].as_str().ok_or("invalid_link")?,
                router["id"].as_str().ok_or("invalid_link")?
            );
            if line != &expected {
                return Err("invalid_link".into());
            }
        }
        if l["kind"] == "declared_uplink" {
            let router = devices
                .iter()
                .find(|n| n["id"] == l["from"] && n["role"] == "router")
                .ok_or("invalid_gateway")?;
            let fw = devices
                .iter()
                .find(|n| n["id"] == l["to"] && n["role"] == "firewall")
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
                    .map(String::as_str)
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
            "host {} {} mac {} vlan {} port {} arp {} {}",
            h["id"].as_str().ok_or("invalid_host")?,
            h["ip"].as_str().ok_or("invalid_host")?,
            h["mac"].as_str().ok_or("invalid_host")?,
            h["vlan"].as_u64().ok_or("invalid_host")?,
            h["port"].as_str().ok_or("invalid_host")?,
            h["ip"].as_str().unwrap(),
            h["mac"].as_str().unwrap()
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
                "offset",
                "limit",
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
        "source_host_id",
        "destination_host_id",
    ] {
        if input.get(key).is_some_and(|v| !v.is_string()) {
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
    let matching: Vec<&Value> = hosts
        .iter()
        .filter(|h| {
            (network.is_empty() || h["network_id"] == network)
                && (zone.is_empty() || h["zone"] == zone)
                && (search.is_empty() || h.to_string().to_lowercase().contains(&search))
        })
        .collect();
    let total = matching.len();
    let offset = input["offset"].as_u64().unwrap_or(0).min(10000) as usize;
    let limit = input["limit"].as_u64().unwrap_or(100).clamp(1, 100) as usize;
    let page: Vec<Value> = matching
        .iter()
        .skip(offset)
        .take(limit)
        .map(|h| (*h).clone())
        .collect();
    let mut nodes: Vec<Value> = Vec::new();
    let mut projected_links = Vec::new();
    if view == "l2" {
        let target = if !network.is_empty() {
            network
        } else {
            page.first()
                .or_else(|| matching.first().copied())
                .map(|h| h["network_id"].as_str().unwrap_or(""))
                .unwrap_or_else(|| {
                    hosts
                        .first()
                        .map(|h| h["network_id"].as_str().unwrap_or(""))
                        .unwrap_or("")
                })
        };
        for device in d["devices"].as_array().unwrap() {
            if device["network_id"] == target && device["role"] == "switch" {
                nodes.push(device.clone());
            }
        }
        nodes.extend(
            d["devices"]
                .as_array()
                .unwrap()
                .iter()
                .filter(|n| n["network_id"] == target && n["role"] == "router")
                .cloned(),
        );
        nodes.extend(page.iter().filter(|h| h["network_id"] == target).cloned());
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
                (network.is_empty() || n["id"] == network) && (zone.is_empty() || n["zone"] == zone)
            })
            .filter_map(|n| n["id"].as_str().map(str::to_owned))
            .collect();
        for n in d["networks"]
            .as_array()
            .unwrap_or(&vec![])
            .iter()
            .filter(|n| selected.contains(n["id"].as_str().unwrap_or("")))
        {
            nodes.push(json!({"id":n["id"],"label":n["subnet"],"role":"network","zone":n["zone"],"network_id":n["id"],"subnet":n["subnet"],"host_count":hosts.iter().filter(|h|h["network_id"]==n["id"]).count(),"evidence":n["evidence"]}));
        }
        nodes.extend(
            d["devices"]
                .as_array()
                .unwrap()
                .iter()
                .filter(|n| {
                    (n["role"] == "router" || n["role"] == "firewall")
                        && (network.is_empty() || n["network_id"] == network)
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
        let mut selected_zones: HashSet<String> = if zone.is_empty() {
            d["zones"]
                .as_array()
                .unwrap()
                .iter()
                .filter_map(Value::as_str)
                .map(str::to_owned)
                .collect()
        } else {
            HashSet::from([zone.to_owned()])
        };
        if !network.is_empty() {
            selected_zones.retain(|z| {
                hosts
                    .iter()
                    .any(|h| h["network_id"] == network && h["zone"].as_str() == Some(z.as_str()))
                    || d["devices"].as_array().unwrap().iter().any(|node| {
                        node["network_id"] == network && node["zone"].as_str() == Some(z.as_str())
                    })
            });
            for link in links.iter().filter(|l| l["kind"] == "declared_uplink") {
                let net_id = link["from"].as_str().unwrap_or("").replace("rtr-", "net-");
                if net_id == network {
                    if let Some(node) = d["devices"]
                        .as_array()
                        .unwrap()
                        .iter()
                        .find(|n| n["id"] == link["to"])
                    {
                        if let Some(z) = node["zone"].as_str() {
                            selected_zones.insert(z.to_owned());
                        }
                    }
                }
            }
        }
        for z in d["zones"].as_array().unwrap() {
            let z = z.as_str().unwrap_or("");
            if selected_zones.contains(z) {
                let evidence: Vec<Value> = d["networks"]
                    .as_array()
                    .unwrap()
                    .iter()
                    .chain(d["devices"].as_array().unwrap())
                    .filter(|n| n["zone"] == z)
                    .flat_map(|n| n["evidence"].as_array().cloned().unwrap_or_default())
                    .collect();
                nodes.push(json!({"id":format!("zone-{z}"),"label":z,"role":"zone","zone":z,"host_count":hosts.iter().filter(|h|h["zone"]==z).count(),"evidence":evidence}));
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
            if a != b
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
            Some(path(hosts, links, src, dst))
        } else {
            None
        }
    } else {
        None
    };
    if view == "l2" {
        if let Some(selected) = &path_result {
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
    let mut result = json!({"schema_version":1,"synthetic":true,"view":view,"counts":{"hosts":hosts.len(),"networks":d["networks"].as_array().map_or(0,Vec::len),"zones":d["zones"].as_array().map_or(0,Vec::len),"visible_nodes":nodes.len(),"visible_links":projected_links.len(),"matched_hosts":total},"nodes":nodes,"links":projected_links,"hosts":page,"pagination":{"offset":offset,"limit":limit,"total":total,"has_more":offset.saturating_add(limit)<total},"sources":sources});
    if let Some(path) = path_result {
        result["path"] = path;
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
            json!({"node_id":id,"ingress":ingress,"egress":egress,"vlan":a["vlan"],"evidence":refs})
        })
        .collect();
    json!({"status":"modeled_l2_path","reason":"explicit_same_vlan_links","hops":hops,"links":ids,"evidence":evidence})
}

#[cfg(test)]
mod tests {
    use super::*;
    #[test]
    fn sample_is_deterministic_and_exactly_one_thousand_hosts() {
        let a = layer_sample(json!({})).unwrap()["dataset"].clone();
        let b = layer_sample(json!({})).unwrap()["dataset"].clone();
        assert_eq!(a, b);
        assert_eq!(a["hosts"].as_array().unwrap().len(), 1000);
        assert_eq!(a["hosts"][999]["id"], "host-1000");
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
    }
}
