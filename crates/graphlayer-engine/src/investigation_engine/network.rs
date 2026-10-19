use super::path::{evaluate_path_inner, evaluate_return_with_forward};
use serde_json::{json, Value};
use std::{
    collections::{HashMap, HashSet},
    net::Ipv4Addr,
    str::FromStr,
};

fn s<'a>(v: &'a Value, k: &str) -> &'a str {
    v.get(k).and_then(Value::as_str).unwrap_or("")
}
fn scope_json(v: &Value) -> Value {
    json!({"snapshot_id":s(v,"snapshot_id"),"device_id":s(v,"device_id"),"context":s(v,"context")})
}
fn flow_json(v: &Value) -> Value {
    let mut f = json!({"scope":scope_json(&v["scope"]),"source":s(v,"source"),"destination":s(v,"destination"),"protocol":s(v,"protocol")});
    for k in ["source_port", "destination_port"] {
        if let Some(n) = v.get(k).and_then(Value::as_i64).filter(|n| *n != 0) {
            f[k] = json!(n)
        }
    }
    f
}
fn endpoint_json(v: &Value) -> Value {
    json!({"node_id":s(v,"node_id"),"interface":s(v,"interface")})
}
fn link_json(v: &Value) -> Value {
    json!({"a":endpoint_json(&v["a"]),"b":endpoint_json(&v["b"])})
}
fn zero_flow() -> Value {
    json!({"scope":{"snapshot_id":"","device_id":"","context":""},"source":"","destination":"","protocol":""})
}
fn arr(v: &Value, k: &str) -> Vec<Value> {
    v.get(k)
        .and_then(Value::as_array)
        .cloned()
        .unwrap_or_default()
}
fn b(v: &Value, k: &str) -> bool {
    v.get(k).and_then(Value::as_bool).unwrap_or(false)
}
fn ip(v: &str) -> Option<u32> {
    Ipv4Addr::from_str(v).ok().map(u32::from)
}
fn prefix(v: &str) -> Option<(u32, u8)> {
    let (addr, bits) = v.split_once('/')?;
    let bits = bits.parse::<u8>().ok()?;
    if bits > 32 {
        return None;
    }
    let addr = ip(addr)?;
    let mask = if bits == 0 {
        0
    } else {
        u32::MAX << (32 - bits)
    };
    Some((addr & mask, bits))
}
fn contains(net: u32, bits: u8, addr: u32) -> bool {
    let mask = if bits == 0 {
        0
    } else {
        u32::MAX << (32 - bits)
    };
    net & mask == addr & mask
}
fn iface(node: &Value, name: &str) -> Option<Value> {
    let matches: Vec<_> = arr(&node["config"]["path"], "interfaces")
        .into_iter()
        .filter(|i| s(i, "name").eq_ignore_ascii_case(name))
        .collect();
    if matches.len() == 1 {
        Some(matches[0].clone())
    } else {
        None
    }
}
fn usable_host(addr: u32, net: u32, bits: u8) -> bool {
    if !contains(net, bits, addr) {
        return false;
    }
    if bits >= 31 {
        return true;
    }
    let host = (1u32 << (32 - bits)) - 1;
    addr != net && addr != net | host
}
fn address_on_interface(node: &Value, name: &str, addr: &str) -> bool {
    let Some(i) = iface(node, name) else {
        return false;
    };
    let Some((net, bits)) = prefix(s(&i, "address")) else {
        return false;
    };
    let Some(addr) = ip(addr) else {
        return false;
    };
    b(&i, "resolved")
        && !b(&i, "shutdown")
        && usable_host(addr, net, bits)
        && ip(s(&i, "address").split('/').next().unwrap_or("")) != Some(addr)
}
pub(super) fn directly_connected(node: &Value, name: &str, addr: &str) -> bool {
    let Some(i) = iface(node, name) else { return false; };
    let Some((net, bits)) = prefix(s(&i, "address")) else { return false; };
    let Some(addr) = ip(addr) else { return false; };
    b(&i, "resolved") && !b(&i, "shutdown") && usable_host(addr, net, bits)
        && arr(&node["config"]["path"], "routes").iter().any(|r| {
            s(r, "kind").eq_ignore_ascii_case("connected")
                && s(r, "interface").eq_ignore_ascii_case(name)
                && prefix(s(r, "prefix")) == Some((net, bits))
        })
}
fn endpoint_eq(e: &Value, node: &str, interface: &str) -> bool {
    s(e, "node_id") == node && s(e, "interface").eq_ignore_ascii_case(interface)
}
fn refs(snapshot: &str, refs: Vec<String>) -> Vec<Value> {
    refs.into_iter()
        .filter(|reference| !reference.is_empty())
        .map(|r| json!({"snapshot_id": snapshot, "ref": r}))
        .collect()
}
fn link_refs(nodes: &HashMap<String, Value>, link: &Value) -> Vec<Value> {
    let mut out = Vec::new();
    for e in [&link["a"], &link["b"]] {
        if let Some(n) = nodes.get(s(e, "node_id")) {
            if let Some(i) = iface(n, s(e, "interface")) {
                out.extend(refs(
                    s(&n["scope"], "snapshot_id"),
                    arr(&i, "evidence_refs")
                        .iter()
                        .filter_map(Value::as_str)
                        .map(str::to_owned)
                        .collect(),
                ));
            }
        }
    }
    out
}
fn append_evidence(hop: &mut Value, evidence: Vec<Value>) {
    if !evidence.is_empty() {
        let fields = hop.as_object_mut().unwrap();
        fields.entry("evidence").or_insert_with(|| json!([])).as_array_mut().unwrap().extend(evidence);
    }
}
fn path_input(node: &Value, ingress: &str, flow: &Value) -> Value {
    json!({"scope": scope_json(&node["scope"]), "config": node["config"], "ingress_interface": ingress, "flow": flow_json(flow)})
}
fn fresh_result(input: &Value, vendor: bool) -> Value {
    json!({"engine_contract_version":if vendor{"graphlayer-network-path-v2"}else{"graphlayer-network-path-v1"},"original_flow":flow_json(&input["flow"]),"forward":{"disposition":"indeterminate","hops":[],"final_flow":zero_flow(),"reasons":[]},"return":{"disposition":"not_evaluated","hops":[],"final_flow":zero_flow(),"reasons":[]},"disposition":"indeterminate","reasons":[],"assumptions":["configuration and topology are supplied evidence, not live device or link state","only static IPv4 forwarding supported by the single-device path model is evaluated","a modeled return leg assumes the reverse TCP/UDP tuple belongs to the established session represented by the forward permit; live connection state is not queried"]})
}
fn uncertain(mut result: Value, reason: &str) -> Value {
    result["forward"]["reasons"]
        .as_array_mut()
        .unwrap()
        .push(json!(reason));
    result["reasons"]
        .as_array_mut()
        .unwrap()
        .push(json!(reason));
    result
}

/// Compose bounded single-firewall evaluations over the explicitly supplied graph.
pub fn evaluate_network(input: Value) -> Result<Value, String> {
    let nodes = arr(&input, "nodes");
    let links = arr(&input, "links");
    let vendor = nodes.iter().any(|n| {
        let v = s(&n["config"], "vendor").trim().to_ascii_lowercase();
        !v.is_empty() && !["cisco-asa", "cisco_asa", "asa"].contains(&v.as_str())
    });
    let mut result = fresh_result(&input, vendor);
    if nodes.is_empty() || nodes.len() > 16 || links.len() > 32 {
        return Ok(uncertain(
            result,
            "topology exceeds supported node or link limits",
        ));
    }
    let source = &input["source"];
    let destination = &input["destination"];
    if s(source, "node_id").is_empty()
        || s(source, "interface").is_empty()
        || s(destination, "node_id").is_empty()
        || s(destination, "interface").is_empty()
        || s(source, "node_id").len() > 64
        || s(destination, "node_id").len() > 64
        || s(source, "interface").len() > 48
        || s(destination, "interface").len() > 48
    {
        return Err("source and destination node/interface are required".into());
    }
    let mut by_id = HashMap::<String, Value>::new();
    for node in &nodes {
        let id = s(node, "id");
        if id.is_empty() || by_id.insert(id.to_owned(), node.clone()).is_some() {
            return Ok(uncertain(
                result,
                "topology contains a missing or duplicate node identifier",
            ));
        }
    }
    let Some(start) = by_id.get(s(source, "node_id")) else {
        return Ok(uncertain(result, "source node is absent from topology"));
    };
    if !by_id.contains_key(s(destination, "node_id")) {
        return Ok(uncertain(
            result,
            "destination node is absent from topology",
        ));
    }
    let mut used = HashSet::new();
    for link in &links {
        if s(&link["a"], "node_id").is_empty()
            || s(&link["b"], "node_id").is_empty()
            || s(&link["a"], "interface").is_empty()
            || s(&link["b"], "interface").is_empty()
            || s(&link["a"], "node_id") == s(&link["b"], "node_id")
        {
            return Ok(uncertain(
                result,
                "topology contains an invalid link endpoint",
            ));
        }
        for e in [&link["a"], &link["b"]] {
            let id = s(e, "node_id");
            let name = s(e, "interface");
            let Some(node) = by_id.get(id) else {
                return Ok(uncertain(result, "link references a missing node"));
            };
            if id.len() > 64 || name.len() > 48 {
                return Ok(uncertain(
                    result,
                    "topology identifier exceeds supported length",
                ));
            }
            if !used.insert(format!("{}\0{}", id, name.to_lowercase())) {
                return Ok(uncertain(
                    result,
                    "interface endpoint is repeated in topology links",
                ));
            }
            let Some(i) = iface(node, name) else {
                return Ok(uncertain(
                    result,
                    "link endpoint interfaces are unresolved or do not share one IPv4 subnet",
                ));
            };
            if !b(&i, "resolved") || b(&i, "shutdown") {
                return Ok(uncertain(
                    result,
                    "link endpoint interfaces are unresolved or do not share one IPv4 subnet",
                ));
            }
        }
        let ia = iface(
            by_id.get(s(&link["a"], "node_id")).unwrap(),
            s(&link["a"], "interface"),
        )
        .unwrap();
        let ib = iface(
            by_id.get(s(&link["b"], "node_id")).unwrap(),
            s(&link["b"], "interface"),
        )
        .unwrap();
        let pa = prefix(s(&ia, "address"));
        let pb = prefix(s(&ib, "address"));
        let aa = ip(s(&ia, "address").split('/').next().unwrap_or(""));
        let ab = ip(s(&ib, "address").split('/').next().unwrap_or(""));
        if pa.is_none()
            || pa != pb
            || aa == ab
            || !aa
                .zip(pa)
                .is_some_and(|(x, (n, bits))| usable_host(x, n, bits))
            || !ab
                .zip(pb)
                .is_some_and(|(x, (n, bits))| usable_host(x, n, bits))
        {
            return Ok(uncertain(
                result,
                "link endpoint interfaces are unresolved or do not share one IPv4 subnet",
            ));
        }
    }
    if scope_json(&input["flow"]["scope"]) != scope_json(&start["scope"]) {
        return Err("flow scope must match the source node scope".into());
    }
    let mut current = source.clone();
    let mut flow = input["flow"].clone();
    let mut visited = HashSet::new();
    for hop in 0..16 {
        let id = s(&current, "node_id").to_owned();
        if !visited.insert(id.clone()) {
            return Ok(uncertain(result, "topology traversal contains a cycle"));
        }
        let node = by_id.get(&id).unwrap();
        if hop == 0 && !address_on_interface(node, s(&current, "interface"), s(&flow, "source")) {
            return Ok(uncertain(
                result,
                "source address is not proven on the selected ingress interface subnet",
            ));
        }
        flow["scope"] = scope_json(&node["scope"]);
        let path = match evaluate_path_inner(
            path_input(node, s(&current, "interface"), &flow),
            None,
            None,
        ) {
            Ok(x) => x,
            Err(e) => {
                return Ok(uncertain(
                    result,
                    &format!("node path inputs could not be evaluated: {e}"),
                ))
            }
        };
        let snap = s(&node["scope"], "snapshot_id");
        let ev = refs(
            snap,
            arr(&path, "evidence_refs")
                .iter()
                .filter_map(Value::as_str)
                .map(str::to_owned)
                .collect(),
        );
        let mut hop_value = json!({"node_id":id,"snapshot_id":snap,"path":path});
        if !ev.is_empty() {
            hop_value["evidence"] = json!(ev)
        }
        result["forward"]["hops"]
            .as_array_mut()
            .unwrap()
            .push(hop_value);
        let last = result["forward"]["hops"].as_array_mut().unwrap().len() - 1;
        if s(&path, "disposition") != "modeled_forward" {
            result["forward"]["disposition"] = json!(s(&path, "disposition"));
            result["forward"]["reasons"] = path["reasons"].clone();
            result["reasons"] = path["reasons"].clone();
            if s(&path, "disposition") == "blocked" {
                result["disposition"] = json!("blocked");
            }
            return Ok(result);
        }
        flow = path["egress_flow"].clone();
        if id == s(destination, "node_id") {
            if !s(&path, "egress_interface").eq_ignore_ascii_case(s(destination, "interface")) {
                return Ok(uncertain(
                    result,
                    "destination node egress differs from requested terminal interface",
                ));
            }
            if !directly_connected(node, s(destination, "interface"), s(&flow, "destination")) {
                return Ok(uncertain(result,"terminal destination is not proven directly connected on requested egress interface"));
            }
            result["forward"]["disposition"] = json!("modeled_forward");
            result["forward"]["final_flow"] = flow.clone();
            let (returns, ret_disposition, ret_reasons, final_flow) = evaluate_return_path(
                &input,
                &by_id,
                &links,
                result["forward"]["hops"].as_array().unwrap(),
            )?;
            result["return"]["hops"] = json!(returns);
            result["return"]["disposition"] = json!(ret_disposition);
            if !ret_reasons.is_empty() {
                result["return"]["reasons"] = json!(ret_reasons);
                result["reasons"] = json!([ret_reasons.last().unwrap()]);
                result["disposition"] = json!(if ret_disposition == "blocked" {
                    "blocked"
                } else {
                    "indeterminate"
                });
            } else {
                result["return"]["final_flow"] = final_flow;
                result["disposition"] = json!("modeled_round_trip");
            }
            return Ok(result);
        }
        let mut selected = None;
        let mut peer = None;
        for link in &links {
            if endpoint_eq(&link["a"], &id, s(&path, "egress_interface")) {
                selected = Some(link.clone());
                peer = Some(link["b"].clone());
                break;
            }
            if endpoint_eq(&link["b"], &id, s(&path, "egress_interface")) {
                selected = Some(link.clone());
                peer = Some(link["a"].clone());
                break;
            }
        }
        let Some(link) = selected else {
            return Ok(uncertain(
                result,
                "selected egress has no explicit topology link",
            ));
        };
        let peer = peer.unwrap();
        let peer_node = by_id.get(s(&peer, "node_id")).unwrap();
        let route = &path["selected_route"];
        let (Some(pi), Some(li)) = (
            iface(peer_node, s(&peer, "interface")),
            iface(node, s(&path, "egress_interface")),
        ) else {
            return Ok(uncertain(
                result,
                "selected route does not prove the linked peer interface as its next hop",
            ));
        };
        let peer_prefix = prefix(s(&pi, "address"));
        let local_prefix = prefix(s(&li, "address"));
        let peer_addr = ip(s(&pi, "address").split('/').next().unwrap_or(""));
        if !s(route, "kind").eq_ignore_ascii_case("static")
            || !s(route, "interface").eq_ignore_ascii_case(s(&path, "egress_interface"))
            || ip(s(route, "next_hop")) != peer_addr
            || peer_prefix != local_prefix
        {
            return Ok(uncertain(
                result,
                "selected route does not prove the linked peer interface as its next hop",
            ));
        }
        result["forward"]["hops"][last]["link"] = link_json(&link);
        append_evidence(&mut result["forward"]["hops"][last], link_refs(&by_id, &link));
        current = peer;
    }
    Ok(uncertain(
        result,
        "topology traversal exceeds supported hop limit",
    ))
}

fn evaluate_return_path(
    input: &Value,
    nodes: &HashMap<String, Value>,
    links: &[Value],
    forward_hops: &[Value],
) -> Result<(Vec<Value>, String, Vec<String>, Value), String> {
    let mut hop_results = Vec::new();
    let mut carried: Option<Value> = None;
    for index in (0..forward_hops.len()).rev() {
        let h = &forward_hops[index];
        let node = nodes.get(s(h, "node_id")).unwrap();
        let mut flow = h["path"]["egress_flow"].clone();
        let source = flow["source"].clone();
        flow["source"] = flow["destination"].clone();
        flow["destination"] = source;
        let source_port = flow["source_port"].as_i64().unwrap_or(0);
        let destination_port = flow["destination_port"].as_i64().unwrap_or(0);
        if destination_port == 0 {
            flow.as_object_mut().unwrap().remove("source_port");
        } else {
            flow["source_port"] = json!(destination_port)
        }
        if source_port == 0 {
            flow.as_object_mut().unwrap().remove("destination_port");
        } else {
            flow["destination_port"] = json!(source_port)
        }
        flow["scope"] = scope_json(&node["scope"]);
        if let Some(mut next) = carried.take() {
            next["scope"] = scope_json(&node["scope"]);
            for k in [
                "source",
                "destination",
                "protocol",
                "source_port",
                "destination_port",
            ] {
                if next[k] != flow[k] {
                    return Ok((hop_results,"indeterminate".into(),vec!["return tuple from the next firewall does not match this firewall's expected reverse tuple".into()],Value::Null));
                }
            }
            flow = next;
        }
        let path = match evaluate_return_with_forward(
            path_input(node, s(&h["path"], "egress_interface"), &flow),
            h["path"].clone(),
        ) {
            Ok(path) => path,
            Err(error) => return Ok((hop_results, "indeterminate".into(), vec![format!("return path is not established on node {}: {error}", s(h, "node_id"))], zero_flow())),
        };
        let snap = s(&node["scope"], "snapshot_id");
        let ev = refs(
            snap,
            arr(&path, "evidence_refs")
                .iter()
                .filter_map(Value::as_str)
                .map(str::to_owned)
                .collect(),
        );
        let mut hop_value = json!({"node_id":s(h,"node_id"),"snapshot_id":snap,"path":path});
        if !ev.is_empty() {
            hop_value["evidence"] = json!(ev)
        }
        hop_results.push(hop_value);
        if s(&path, "disposition") != "modeled_forward" {
            let disp = s(&path, "disposition").to_owned();
            let mut reasons: Vec<String> = arr(&path, "reasons").iter().filter_map(Value::as_str).map(str::to_owned).collect();
            reasons.push(format!("return path is blocked or indeterminate on node {}", s(h, "node_id")));
            return Ok((
                hop_results,
                disp,
                reasons,
                Value::Null,
            ));
        }
        if index == 0 {
            if !address_on_interface(
                node,
                s(&input["source"], "interface"),
                s(&path["egress_flow"], "destination"),
            ) {
                return Ok((
                    hop_results,
                    "indeterminate".into(),
                    vec![
                        "return destination is not proven on the requested source interface subnet"
                            .into(),
                    ],
                    Value::Null,
                ));
            }
            return Ok((
                hop_results,
                "modeled_forward".into(),
                vec![],
                path["egress_flow"].clone(),
            ));
        }
        let prev = &forward_hops[index - 1];
        let mut reverse_link = None;
        for link in links {
            let forward = (endpoint_eq(&link["a"], s(h, "node_id"), s(&path, "egress_interface"))
                && endpoint_eq(
                    &link["b"],
                    s(prev, "node_id"),
                    s(&prev["path"], "egress_interface"),
                ))
                || (endpoint_eq(&link["b"], s(h, "node_id"), s(&path, "egress_interface"))
                    && endpoint_eq(
                        &link["a"],
                        s(prev, "node_id"),
                        s(&prev["path"], "egress_interface"),
                    ));
            if forward {
                reverse_link = Some(link.clone());
                break;
            }
        }
        let Some(link) = reverse_link else {
            return Ok((
                hop_results,
                "indeterminate".into(),
                vec!["return egress does not use the explicit reverse topology link".into()],
                Value::Null,
            ));
        };
        let peer = if s(&link["a"], "node_id") == s(h, "node_id") {
            &link["b"]
        } else {
            &link["a"]
        };
        let peer_node = nodes.get(s(prev, "node_id")).unwrap();
        let (Some(local), Some(peer_i)) = (
            iface(node, s(&path, "egress_interface")),
            iface(peer_node, s(peer, "interface")),
        ) else {
            return Ok((
                hop_results,
                "indeterminate".into(),
                vec![
                    "return route does not prove the linked peer interface as its next hop".into(),
                ],
                Value::Null,
            ));
        };
        let route = &path["selected_route"];
        let peer_addr = ip(s(&peer_i, "address").split('/').next().unwrap_or(""));
        if !s(route, "kind").eq_ignore_ascii_case("static")
            || !s(route, "interface").eq_ignore_ascii_case(s(&path, "egress_interface"))
            || ip(s(route, "next_hop")) != peer_addr
            || prefix(s(&local, "address")) != prefix(s(&peer_i, "address"))
        {
            return Ok((
                hop_results,
                "indeterminate".into(),
                vec![
                    "return route does not prove the linked peer interface as its next hop".into(),
                ],
                Value::Null,
            ));
        }
        let last = hop_results.len() - 1;
        hop_results[last]["link"] = link_json(&link);
        append_evidence(&mut hop_results[last], link_refs(nodes, &link));
        carried = Some(path["egress_flow"].clone());
    }
    Ok((
        hop_results,
        "indeterminate".into(),
        vec!["return path did not reach its source".into()],
        Value::Null,
    ))
}
