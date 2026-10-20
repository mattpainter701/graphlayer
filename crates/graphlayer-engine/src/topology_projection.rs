//! Bounded, evidence-linked projection for the topology view. This module only
//! reshapes parsed snapshots and trusted network results; it never reads raw
//! configurations or changes saved identities.
use serde_json::{json, Map, Value};
use std::collections::{BTreeMap, BTreeSet, HashMap, HashSet, VecDeque};
use std::net::Ipv4Addr;
use std::str::FromStr;

const MAX_NODES: usize = 16;
const MAX_LINKS: usize = 32;
const MAX_BYTES: usize = 160 << 20;
const ROLES: &[&str] = &["firewall", "router", "switch", "host", "server", "network"];

fn s<'a>(v: &'a Value, key: &str) -> &'a str { v.get(key).and_then(Value::as_str).unwrap_or("") }
fn a(v: &Value, key: &str) -> Vec<Value> { v.get(key).and_then(Value::as_array).cloned().unwrap_or_default() }
fn arr<'a>(v: &'a Value, key: &str) -> &'a [Value] { v.get(key).and_then(Value::as_array).map(Vec::as_slice).unwrap_or(&[]) }
fn valid_role(role: &str) -> bool { ROLES.contains(&role) }
fn firewall(vendor: &str) -> bool { matches!(vendor.trim().to_ascii_lowercase().as_str(), "cisco-asa"|"cisco_asa"|"asa"|"paloalto-panos"|"fortinet-fortigate") }
fn refs(snapshot: &str, interface: &Value, known: &HashMap<String,String>) -> Vec<Value> {
    a(interface, "evidence_refs").iter().filter_map(Value::as_str).filter(|r| known.contains_key(*r))
        .map(|r| json!({"snapshot_id":snapshot,"ref":r})).collect()
}
fn known_evidence(config: &Value) -> HashMap<String,String> {
    arr(config,"evidence").iter().filter_map(|e| { let id=s(e,"id"); (!id.is_empty()).then(||(id.to_owned(),s(e,"kind").to_owned())) }).collect()
}
fn interface_fact_supported(interface: &Value, evidence: &HashMap<String,String>) -> bool {
    a(interface,"evidence_refs").iter().filter_map(Value::as_str).any(|id| matches!(evidence.get(id).map(String::as_str),Some("interface"|"path_interface")))
}
fn tuple(value: &Value) -> Value {
    let mut out=json!({"source":s(value,"source"),"destination":s(value,"destination"),"protocol":s(value,"protocol")});
    if let Some(scope)=value.get("scope") { out["scope"]=json!({"snapshot_id":s(scope,"snapshot_id"),"device_id":s(scope,"device_id"),"context":s(scope,"context")}); }
    for key in ["source_port","destination_port"] { if let Some(port)=value.get(key).and_then(Value::as_u64).filter(|p| (1..=65535).contains(p)) { out[key]=json!(port); } }
    out
}
fn select_saved_result(input: &Value) -> Result<(Value, Value), String> {
    let null=Value::Null;
    let result=input.get("result").unwrap_or(&null);
    let branch_index=input.get("branch_index").map(|v|v.as_u64().ok_or("invalid_request")).transpose()?.unwrap_or(0) as usize;
    let return_index=input.get("return_index").map(|v|v.as_u64().ok_or("invalid_request")).transpose()?.unwrap_or(0) as usize;
    if result.is_null() {
        if branch_index!=0 || return_index!=0 { return Err("invalid_request".into()); }
        return Ok((Value::Null,Value::Null));
    }
    if let Some(branches)=result.get("branches").and_then(Value::as_array) {
        if branches.is_empty() {
            if branch_index!=0 || return_index!=0 { return Err("invalid_request".into()); }
            return Ok((result.clone(),Value::Null));
        }
        let Some(branch)=branches.get(branch_index) else { return Err("invalid_request".into()); };
        let returns=arr(branch,"returns");
        let selected_return=if returns.is_empty() { if return_index!=0{return Err("invalid_request".into());} Value::Null }
            else { let Some(ret)=returns.get(return_index) else {return Err("invalid_request".into());}; ret.clone() };
        return Ok((branch.clone(),selected_return));
    }
    if branch_index!=0 || return_index!=0 { return Err("invalid_request".into()); }
    let selected_return=result.get("return").filter(|v|!v.is_null()).cloned().unwrap_or(Value::Null);
    Ok((result.clone(),selected_return))
}
fn iface<'a>(node: &'a Value, name: &str) -> Option<&'a Value> {
    let xs = arr(&node["config"]["path"], "interfaces");
    let mut found = xs.iter().filter(|i| s(i,"name").eq_ignore_ascii_case(name));
    let first = found.next()?;
    if found.next().is_some() { None } else { Some(first) }
}
fn subnet(address: &str) -> Option<String> {
    let (ip, bits) = address.split_once('/')?;
    let ip = u32::from(Ipv4Addr::from_str(ip).ok()?);
    let bits: u8 = bits.parse().ok()?;
    if bits > 32 { return None; }
    let mask = if bits == 0 { 0 } else { u32::MAX << (32-bits) };
    Some(format!("{}/{}", Ipv4Addr::from(ip & mask), bits))
}
fn add_facet(facets: &mut BTreeMap<String, usize>, value: &str) {
    if !value.is_empty() { *facets.entry(value.to_owned()).or_default() += 1; }
}
fn facet(values: BTreeMap<String,usize>) -> Value {
    let list: Vec<Value> = values.into_iter().map(|(value,count)| json!({"value":value,"count":count})).collect();
    json!({"available":!list.is_empty(),"values":list})
}
fn clean_string(value: &str, max: usize) -> bool { !value.is_empty() && value.len() <= max && !value.chars().any(char::is_control) }
fn ipv4_literal(value: &str) -> bool { Ipv4Addr::from_str(value).is_ok() }
fn bounded_size(value: &Value, limit: usize) -> usize {
    fn escaped_len(value: &str) -> usize {
        value.chars().map(|c| match c { '"'|'\\' => 2, c if c <= '\u{1f}' => 6, _ => c.len_utf8() }).sum()
    }
    fn visit(v: &Value, size: &mut usize, limit: usize) {
        if *size >= limit { return; }
        match v {
            Value::Null => *size=size.saturating_add(4),
            Value::Bool(_) => *size=size.saturating_add(5),
            Value::Number(n) => *size=size.saturating_add(n.to_string().len()),
            Value::String(s) => *size=size.saturating_add(escaped_len(s)).saturating_add(2),
            Value::Array(items) => { *size=size.saturating_add(2); for (index,item) in items.iter().enumerate() { if index>0 { *size=size.saturating_add(1); } visit(item,size,limit); if *size>=limit {break;} } },
            Value::Object(fields) => { *size=size.saturating_add(2); for (index,(key,item)) in fields.iter().enumerate() { if index>0 { *size=size.saturating_add(1); } *size=size.saturating_add(escaped_len(key)).saturating_add(3);visit(item,size,limit);if *size>=limit{break;} } },
        }
    }
    let mut size=0; visit(value,&mut size,limit); size.min(limit)
}
fn modeled_direction(result: &Value, a_node: &str, a_if: &str, b_node: &str, b_if: &str) -> Option<&'static str> {
    let hops=arr(&result["forward"],"hops");
    for pair in hops.windows(2) {
        let left=&pair[0];let right=&pair[1];
        if s(left,"node_id")==a_node && s(&left["path"],"egress_interface").eq_ignore_ascii_case(a_if)
            && s(right,"node_id")==b_node && s(&right["path"],"ingress_interface").eq_ignore_ascii_case(b_if) { return Some("a_to_b"); }
        if s(left,"node_id")==b_node && s(&left["path"],"egress_interface").eq_ignore_ascii_case(b_if)
            && s(right,"node_id")==a_node && s(&right["path"],"ingress_interface").eq_ignore_ascii_case(a_if) { return Some("b_to_a"); }
    }
    None
}
fn path_lines_vr(node: &Value) -> HashMap<String, (String, Vec<String>)> {
    let config = &node["config"];
    let mut line_refs = HashMap::<i64, Vec<String>>::new();
    for ev in arr(config,"evidence") {
        if let Some(line) = ev.get("start_line").and_then(Value::as_i64) {
            if s(ev,"kind") == "virtual_router_membership" {
                line_refs.entry(line).or_default().push(s(ev,"id").to_owned());
            }
        }
    }
    let mut candidates = HashMap::<String,HashSet<String>>::new();
    let mut evidence_by_interface = HashMap::<String,HashSet<String>>::new();
    for line in arr(config,"source_lines") {
        let Some(number) = line.get("number").and_then(Value::as_i64) else { continue; };
        let text = s(line,"text").trim();
        let fields: Vec<_> = text.split_whitespace().collect();
        if fields.len() == 6 && fields[0] == "set" && fields[1] == "network" && fields[2] == "virtual-router" && fields[4] == "interface" {
            let line_evidence = line_refs.get(&number).cloned().unwrap_or_default();
            if !line_evidence.is_empty() && clean_string(fields[3], 128) && clean_string(fields[5], 48) {
                let interface=fields[5].to_ascii_lowercase();
                candidates.entry(interface.clone()).or_default().insert(fields[3].to_owned());
                evidence_by_interface.entry(interface).or_default().extend(line_evidence);
            }
        }
    }
    candidates.into_iter().filter_map(|(interface,names)| {
        if names.len()!=1 { return None; }
        let name=names.into_iter().next()?;
        let line_refs=evidence_by_interface.remove(&interface).unwrap_or_default();
        let actual_refs=a(&node["config"]["path"],"interfaces").into_iter()
            .find(|i|s(i,"name").eq_ignore_ascii_case(&interface))
            .map(|i|a(&i,"evidence_refs").into_iter().filter_map(|r|r.as_str().map(str::to_owned)).collect::<HashSet<_>>())
            .unwrap_or_default();
        let owned_refs:Vec<String>=line_refs.intersection(&actual_refs).cloned().collect();
        (!owned_refs.is_empty()).then_some((interface,(name,owned_refs)))
    }).collect()
}

/// Project existing network requests and parsed snapshots into view-only facts.
pub fn project_topology(input: Value) -> Result<Value, String> {
    let request = input.get("request").ok_or("invalid_request")?;
    if bounded_size(&input,MAX_BYTES.saturating_add(1)) > MAX_BYTES { return Err("topology projection exceeds input budget".into()); }
    let nodes = arr(request,"nodes");
    let links = arr(request,"links");
    if nodes.is_empty() || nodes.len() > MAX_NODES || links.len() > MAX_LINKS { return Err("topology exceeds supported node or link limits".into()); }
    let roles = input.get("roles").and_then(Value::as_object).cloned().unwrap_or_default();
    if input.get("roles").is_some_and(|v| !v.is_object()) { return Err("invalid roles".into()); }
    let (result,selected_return)=select_saved_result(&input)?;
    let mut by_id = HashMap::<String, &Value>::new();
    for n in nodes {
        let id = s(n,"id");
        if !clean_string(id,64) || id.starts_with("@query-") || by_id.insert(id.to_owned(),n).is_some() { return Err("invalid or duplicate topology node identifier".into()); }
    }
    let null = Value::Null;
    let flow = request.get("flow").unwrap_or(&null);
    let has_query_source = ipv4_literal(s(flow,"source")) && !s(&request["source"],"node_id").is_empty();
    let has_query_destination = ipv4_literal(s(flow,"destination")) && !s(&request["destination"],"node_id").is_empty();
    for (id, role) in &roles {
        let endpoint = (id == "@query-source" && has_query_source) || (id == "@query-destination" && has_query_destination);
        if (!by_id.contains_key(id) && !endpoint) || !role.as_str().is_some_and(valid_role) { return Err("invalid declared topology role".into()); }
    }
    for l in links {
        for endpoint in [&l["a"], &l["b"]] {
            let Some(node)=by_id.get(s(endpoint,"node_id")) else { return Err("invalid topology link endpoint".into()); };
            if !clean_string(s(endpoint,"interface"),48) || iface(node,s(endpoint,"interface")).is_none() { return Err("invalid topology link endpoint".into()); }
        }
    }
    for endpoint in [&request["source"],&request["destination"]] {
        if !endpoint.is_null() && !s(endpoint,"node_id").is_empty() {
            let Some(node)=by_id.get(s(endpoint,"node_id")) else { return Err("invalid selected endpoint".into()); };
            if !clean_string(s(endpoint,"interface"),48) || iface(node,s(endpoint,"interface")).is_none() { return Err("invalid selected endpoint".into()); }
        }
    }

    let mut zone_facets = BTreeMap::new(); let mut subnet_facets = BTreeMap::new();
    let mut vrf_facets = BTreeMap::new(); let mut vdom_facets = BTreeMap::new();
    let mut projected = Vec::new();
    for n in nodes {
        let id = s(n,"id"); let config = &n["config"]; let snap = s(&n["scope"],"snapshot_id");
        if !clean_string(snap,128) || !clean_string(s(&n["scope"],"device_id"),128) || !clean_string(s(&n["scope"],"context"),128) { return Err("invalid topology node scope".into()); }
        let vendor = s(config,"vendor");
        let evidence_ids=known_evidence(config);
        let path_interfaces=arr(&config["path"],"interfaces");
        if path_interfaces.len()>1024 { return Err("snapshot interface count exceeds projection limit".into()); }
        let declared = roles.get(id).and_then(Value::as_str);
        let (role, role_source) = if let Some(r) = declared { (r, "operator_annotation") } else if firewall(vendor) { ("firewall", "parsed_vendor") } else { ("device", "unspecified") };
        let icon = if matches!(role,"firewall"|"router"|"switch"|"host"|"server"|"cloud") { role } else { "device" };
        let vr_lines = path_lines_vr(n);
        let mut interfaces = Vec::new(); let mut node_zone = String::new(); let mut node_subnet = String::new();
        let mut node_vrf = String::new(); let mut node_vdom = String::new();
        let mut device_zones=HashSet::new(); let mut device_subnets=HashSet::new(); let mut device_vrfs=HashSet::new(); let mut device_vdoms=HashSet::new();
        for i in path_interfaces {
            let name=s(i,"name"); if !clean_string(name,48) { continue; }
            let evidence = refs(snap,i,&evidence_ids);
            let addr = s(i,"address");
            let interface_supported=interface_fact_supported(i,&evidence_ids);
            let subnet_value = if interface_supported { subnet(addr).unwrap_or_default() } else { String::new() };
            let zone = if interface_supported && clean_string(s(i,"zone"),128) { s(i,"zone") } else { "" };
            let vr = vr_lines.get(&name.to_ascii_lowercase());
            let vrf = vr.map(|x| x.0.as_str()).unwrap_or("");
            let mut if_vdom = "";
            let mut vdom_source = "unavailable";
            if interface_supported && clean_string(s(i,"vdom"),128) { if_vdom=s(i,"vdom"); vdom_source="configured"; }
            else if vendor == "fortinet-fortigate" && s(config,"context").eq_ignore_ascii_case("root") && interface_supported { if_vdom="root"; vdom_source="context"; }
            if node_zone.is_empty() { node_zone=zone.into(); } if node_subnet.is_empty(){node_subnet=subnet_value.clone();}
            if node_vrf.is_empty(){node_vrf=vrf.into();} if node_vdom.is_empty(){node_vdom=if_vdom.into();}
            if !zone.is_empty(){device_zones.insert(zone.to_owned());} if !subnet_value.is_empty(){device_subnets.insert(subnet_value.clone());} if !vrf.is_empty(){device_vrfs.insert(vrf.to_owned());} if !if_vdom.is_empty(){device_vdoms.insert(if_vdom.to_owned());}
            let refs_strings:Vec<String> = evidence.iter().filter_map(|x|x["ref"].as_str().map(str::to_owned)).collect();
            let address_supported=interface_supported && !subnet_value.is_empty();
            let mut entry=json!({"name":name,"address":if address_supported{json!(addr)}else{Value::Null},"subnet":if subnet_value.is_empty(){Value::Null}else{json!(subnet_value)},"zone":if zone.is_empty(){Value::Null}else{json!(zone)},"vrf":if vrf.is_empty(){Value::Null}else{json!(vrf)},"vdom":if if_vdom.is_empty(){Value::Null}else{json!(if_vdom)},"vdom_source":vdom_source,"evidence_refs":refs_strings});
            if let Some((_,vr_refs))=vr { entry["evidence_refs"]=json!(refs_strings.iter().cloned().chain(vr_refs.iter().cloned()).collect::<BTreeSet<_>>().into_iter().collect::<Vec<_>>()); }
            interfaces.push(entry);
        }
        for value in device_zones { add_facet(&mut zone_facets,&value); }
        for value in device_subnets { add_facet(&mut subnet_facets,&value); }
        for value in device_vrfs { add_facet(&mut vrf_facets,&value); }
        for value in device_vdoms { add_facet(&mut vdom_facets,&value); }
        let hostname=s(config,"hostname");
        projected.push(json!({"node_id":id,"label":if clean_string(hostname,128){hostname}else{id},"role":role,"role_source":role_source,"icon":icon,"zone":if node_zone.is_empty(){Value::Null}else{json!(node_zone)},"subnet":if node_subnet.is_empty(){Value::Null}else{json!(node_subnet)},"vrf":if node_vrf.is_empty(){Value::Null}else{json!(node_vrf)},"vdom":if node_vdom.is_empty(){Value::Null}else{json!(node_vdom)},"layout":{"column":0,"row":0},"interfaces":interfaces}));
    }
    // Deterministic graph layers rooted at the explicitly requested source node.
    let mut adjacency: HashMap<String,Vec<String>>=HashMap::new();
    for l in links { let x=s(&l["a"],"node_id").to_owned(); let y=s(&l["b"],"node_id").to_owned(); adjacency.entry(x.clone()).or_default().push(y.clone()); adjacency.entry(y).or_default().push(x); }
    let source_id=s(&request["source"],"node_id"); let mut depth=HashMap::<String,usize>::new(); let mut q=VecDeque::new();
    if by_id.contains_key(source_id){depth.insert(source_id.to_owned(),0);q.push_back(source_id.to_owned());}
    while let Some(id)=q.pop_front(){let d=depth[&id];for next in adjacency.get(&id).into_iter().flatten(){if !depth.contains_key(next){depth.insert(next.clone(),d.saturating_add(1).min(15));q.push_back(next.clone());}}}
    let mut rows=HashMap::<usize,usize>::new();
    for node in &mut projected { let id=s(node,"node_id").to_owned(); let col=*depth.get(&id).unwrap_or(&0);let row=rows.entry(col).or_default();node["layout"]=json!({"column":col,"row":*row});*row+=1; }

    let mut projected_links=Vec::new();
    for (index,l) in links.iter().enumerate(){let ea=&l["a"];let eb=&l["b"];let na=by_id[s(ea,"node_id")];let nb=by_id[s(eb,"node_id")];let mut ev=refs(s(&na["scope"],"snapshot_id"),iface(na,s(ea,"interface")).unwrap_or(&Value::Null),&known_evidence(&na["config"]));ev.extend(refs(s(&nb["scope"],"snapshot_id"),iface(nb,s(eb,"interface")).unwrap_or(&Value::Null),&known_evidence(&nb["config"])));
        let mut unique=HashSet::new();ev.retain(|x|unique.insert((s(x,"snapshot_id").to_owned(),s(x,"ref").to_owned())));
        let direction=modeled_direction(result,s(ea,"node_id"),s(ea,"interface"),s(eb,"node_id"),s(eb,"interface"));
        projected_links.push(json!({"id":format!("link-{index}"),"a":{"node_id":s(ea,"node_id"),"interface":s(ea,"interface")},"b":{"node_id":s(eb,"node_id"),"interface":s(eb,"interface")},"ports":[s(ea,"interface"),s(eb,"interface")],"direction":direction.unwrap_or("bidirectional"),"direction_source":if direction.is_some(){"modeled_path"}else{"unspecified"},"provenance":"configured_link","synthetic":false,"evidence_refs":ev}));
    }
    let result_disposition=s(result,"disposition");
    let mut traffic=json!({"available":false,"status":"unavailable","reason":"No supported traffic telemetry is present"});
    if !result_disposition.is_empty(){traffic=json!({"available":false,"status":result_disposition,"reason":"Modeled path result; no live traffic telemetry"});}
    if !selected_return.is_null() { let direction=selected_return.get("direction").unwrap_or(&selected_return); traffic["selected_return"]=json!({"direction":tuple(&direction["final_flow"]),"disposition":s(&selected_return,"disposition"),"session_status":s(&selected_return,"session_status")}); }
    if !result.is_null() { traffic["selection"]=json!({"branch_index":input.get("branch_index").and_then(Value::as_u64).unwrap_or(0),"return_index":input.get("return_index").and_then(Value::as_u64).unwrap_or(0)}); }
    if let Some(flow)=request.get("flow") { traffic["query_flow"]=tuple(flow); traffic["original_flow"]=tuple(result.get("original_flow").unwrap_or(flow)); if result.get("forward").and_then(|x|x.get("final_flow")).is_some(){traffic["modeled_final_flow"]=tuple(&result["forward"]["final_flow"]);} }
    for endpoint in ["source","destination"] { if request.get(endpoint).is_some_and(Value::is_object) { traffic[endpoint]=json!({"node_id":s(&request[endpoint],"node_id"),"interface":s(&request[endpoint],"interface")}); } }
    let null_flow=Value::Null;
    let flow=request.get("flow").unwrap_or(&null_flow);
    let query_src=s(flow,"source"); let query_dst=s(flow,"destination");
    let source_endpoint=&request["source"]; let destination_endpoint=&request["destination"];
    if ipv4_literal(query_src) && !s(source_endpoint,"node_id").is_empty() {
        let role=roles.get("@query-source").and_then(Value::as_str).unwrap_or("host"); let source=if roles.contains_key("@query-source"){"operator_annotation"}else{"query_endpoint"}; let icon=if matches!(role,"firewall"|"router"|"switch"|"host"|"server"|"cloud"){role}else{"device"};
        projected.push(json!({"node_id":"@query-source","label":query_src,"role":role,"role_source":source,"icon":icon,"address":query_src,"layout":{"column":0,"row":0},"interfaces":[]}));
        let link_id=format!("query-source-{}",s(source_endpoint,"node_id"));
        projected_links.push(json!({"id":link_id,"a":{"node_id":"@query-source","interface":""},"b":{"node_id":s(source_endpoint,"node_id"),"interface":s(source_endpoint,"interface")},"ports":["",s(source_endpoint,"interface")],"direction":"source_to_device","provenance":"query_endpoint","synthetic":true,"evidence_refs":[]}));
    }
    if ipv4_literal(query_dst) && !s(destination_endpoint,"node_id").is_empty() {
        let role=roles.get("@query-destination").and_then(Value::as_str).unwrap_or("host"); let source=if roles.contains_key("@query-destination"){"operator_annotation"}else{"query_endpoint"}; let icon=if matches!(role,"firewall"|"router"|"switch"|"host"|"server"|"cloud"){role}else{"device"};
        projected.push(json!({"node_id":"@query-destination","label":query_dst,"role":role,"role_source":source,"icon":icon,"address":query_dst,"layout":{"column":0,"row":0},"interfaces":[]}));
        let link_id=format!("query-destination-{}",s(destination_endpoint,"node_id"));
        projected_links.push(json!({"id":link_id,"a":{"node_id":s(destination_endpoint,"node_id"),"interface":s(destination_endpoint,"interface")},"b":{"node_id":"@query-destination","interface":""},"ports":[s(destination_endpoint,"interface"),""],"direction":"device_to_destination","provenance":"query_endpoint","synthetic":true,"evidence_refs":[]}));
    }
    let mut max_col=0usize; for n in &projected { max_col=max_col.max(n["layout"]["column"].as_u64().unwrap_or(0) as usize); }
    for n in &mut projected { let id=s(n,"node_id"); if id=="@query-source" {n["layout"]=json!({"column":0,"row":0});} else if id=="@query-destination" {n["layout"]=json!({"column":max_col+2,"row":0});} else {let c=n["layout"]["column"].as_u64().unwrap_or(0) as usize;n["layout"]["column"]=json!(c+if has_query_source {1} else {0});} }
    let topology=json!({"nodes":projected,"links":projected_links,"facets":{"zone":facet(zone_facets),"subnet":facet(subnet_facets),"vrf":facet(vrf_facets),"vdom":facet(vdom_facets)},"traffic":traffic});
    let mut out=Map::new(); out.insert("topology".into(),topology); Ok(Value::Object(out))
}

#[cfg(test)]
mod tests {
    use super::*;
    fn input() -> Value {
        json!({"request":{"nodes":[{"id":"fw-a","scope":{"snapshot_id":"snap-a","device_id":"dev-a","context":"default"},"config":{"vendor":"cisco-asa","hostname":"edge-a","evidence":[{"id":"ev-if","start_line":1,"kind":"interface"}],"path":{"interfaces":[{"name":"inside","address":"10.0.1.3/24","resolved":true,"zone":"trust","evidence_refs":["ev-if"]}]},"source_lines":[{"number":1,"text":"password secret-not-for-view"}]}}],"links":[],"source":{"node_id":"fw-a","interface":"inside"},"destination":{"node_id":"fw-a","interface":"inside"},"flow":{"scope":{"snapshot_id":"snap-a","device_id":"dev-a","context":"default"},"source":"10.0.1.10","destination":"10.0.1.20","protocol":"tcp","destination_port":443}},"result":{"disposition":"modeled_forward","forward":{"final_flow":{"source":"10.0.1.10","destination":"10.0.1.20"}}}})
    }
    #[test]
    fn facets_are_evidence_backed_and_subnet_is_normalized() {
        let mut sample=input();
        let second=sample["request"]["nodes"][0]["config"]["path"]["interfaces"][0].clone();
        sample["request"]["nodes"][0]["config"]["path"]["interfaces"].as_array_mut().unwrap().push(second);
        let projected=project_topology(sample).unwrap(); let top=&projected["topology"];
        assert_eq!(top["facets"]["zone"]["values"][0]["value"],"trust");
        assert_eq!(top["facets"]["zone"]["values"][0]["count"],1);
        assert_eq!(top["facets"]["subnet"]["values"][0]["value"],"10.0.1.0/24");
        assert_eq!(top["nodes"][0]["interfaces"][0]["evidence_refs"][0],"ev-if");
    }
    #[test]
    fn vrf_and_vdom_are_unavailable_without_modeled_evidence() {
        let top=project_topology(input()).unwrap();
        assert_eq!(top["topology"]["facets"]["vrf"]["available"],false);
        assert_eq!(top["topology"]["facets"]["vdom"]["available"],false);
    }
    #[test]
    fn panos_virtual_router_name_requires_matching_parser_evidence() {
        let mut sample=input();
        let config=&mut sample["request"]["nodes"][0]["config"];
        config["vendor"]=json!("paloalto-panos");
        config["source_lines"]=json!([{"number":9,"text":"set network virtual-router blue interface ethernet1/1"}]);
        config["evidence"]=json!([{"id":"ev-vr","start_line":9,"kind":"virtual_router_membership"}]);
        config["path"]["interfaces"][0]["evidence_refs"]=json!(["ev-vr"]);
        let top=project_topology(sample).unwrap();
        assert_eq!(top["topology"]["facets"]["vrf"]["values"][0]["value"],"blue");
        config["evidence"]=json!([]);
        let top=project_topology(sample).unwrap();
        assert_eq!(top["topology"]["facets"]["vrf"]["available"],false);
    }
    #[test]
    fn panos_virtual_router_evidence_must_belong_to_interface_and_conflicts_are_unavailable() {
        let mut sample=input();
        let config=&mut sample["request"]["nodes"][0]["config"];
        config["vendor"]=json!("paloalto-panos");
        config["source_lines"]=json!([
            {"number":9,"text":"set network virtual-router blue interface inside"},
            {"number":10,"text":"set network virtual-router red interface inside"}
        ]);
        config["evidence"]=json!([
            {"id":"ev-vr-a","start_line":9,"kind":"virtual_router_membership"},
            {"id":"ev-vr-b","start_line":10,"kind":"virtual_router_membership"},
            {"id":"ev-if","start_line":1,"kind":"interface"}
        ]);
        config["path"]["interfaces"][0]["evidence_refs"]=json!(["ev-if"]);
        let detached=project_topology(sample.clone()).unwrap();
        assert_eq!(detached["topology"]["facets"]["vrf"]["available"],false);
        config["path"]["interfaces"][0]["evidence_refs"]=json!(["ev-if","ev-vr-a","ev-vr-b"]);
        let conflicting=project_topology(sample).unwrap();
        assert_eq!(conflicting["topology"]["facets"]["vrf"]["available"],false);
    }
    #[test]
    fn fortios_root_context_is_labeled_as_context_provenance() {
        let mut sample=input();
        let config=&mut sample["request"]["nodes"][0]["config"];
        config["vendor"]=json!("fortinet-fortigate"); config["context"]=json!("root");
        let top=project_topology(sample).unwrap();
        assert_eq!(top["topology"]["facets"]["vdom"]["values"][0]["value"],"root");
        assert_eq!(top["topology"]["nodes"][0]["interfaces"][0]["vdom_source"],"context");
    }
    #[test]
    fn declared_roles_are_validated_and_only_projection_fields_are_emitted() {
        let mut req=input(); req["roles"]=json!({"fw-a":"router"});
        let top=project_topology(req).unwrap();
        assert_eq!(top["topology"]["nodes"][0]["role"],"router");
        let serialized=top.to_string();
        assert!(!serialized.contains("secret-not-for-view"));
        let mut bad=input(); bad["roles"]=json!({"fw-a":"database"});
        assert!(project_topology(bad).is_err());
        let mut unknown=input(); unknown["roles"]=json!({"not-a-node":"host"});
        assert!(project_topology(unknown).is_err());
    }
    #[test]
    fn query_endpoints_are_explicit_and_do_not_claim_live_telemetry() {
        let top=project_topology(input()).unwrap(); let top=&top["topology"];
        assert_eq!(top["nodes"].as_array().unwrap().len(),3);
        assert_eq!(top["nodes"][1]["role_source"],"query_endpoint");
        assert_eq!(top["links"][0]["provenance"],"query_endpoint");
        assert_eq!(top["traffic"]["available"],false);
    }
    #[test]
    fn malformed_and_oversized_requests_fail_closed() {
        assert!(project_topology(json!({"request":{}})).is_err());
        let mut too_many=input(); too_many["request"]["nodes"]=json!((0..17).map(|i|json!({"id":format!("n{i}"),"scope":{},"config":{}})).collect::<Vec<_>>());
        assert!(project_topology(too_many).is_err());
        assert_eq!(bounded_size(&json!({"x":"large"}),1),1);
        let mut bad_link=input(); bad_link["request"]["links"]=json!([{"a":{"node_id":"missing","interface":"x"},"b":{"node_id":"fw-a","interface":"inside"}}]);
        assert!(project_topology(bad_link).is_err());
    }
    #[test]
    fn bounded_size_accounts_for_array_and_object_separators() {
        assert_eq!(bounded_size(&json!([1,2]),100),5);
        assert_eq!(bounded_size(&json!({"a":1,"b":2}),100),13);
    }
    #[test]
    fn only_evidence_owned_by_snapshot_and_whitelisted_tuple_fields_are_returned() {
        let mut sample=input();
        sample["request"]["nodes"][0]["config"]["path"]["interfaces"][0]["evidence_refs"]=json!(["foreign-ref"]);
        sample["request"]["flow"]["password"]=json!("do-not-return");
        sample["request"]["flow"]["source_port"]=json!(99999);
        sample["result"]["credential"]=json!("do-not-return-either");
        let output=project_topology(sample).unwrap().to_string();
        assert!(!output.contains("foreign-ref"));
        assert!(!output.contains("do-not-return"));
        assert!(!output.contains("source_port"));
    }
    #[test]
    fn saved_exploration_branch_and_return_selection_is_bounded_in_rust() {
        let mut sample=input();
        sample["result"]=json!({"disposition":"modeled_round_trip","branches":[
            {"disposition":"modeled_round_trip","forward":{"disposition":"modeled_forward","hops":[],"final_flow":{"source":"10.0.1.10","destination":"10.0.1.20","protocol":"tcp"}},"returns":[{"disposition":"modeled_forward","session_status":"established","direction":{"disposition":"modeled_forward","hops":[],"final_flow":{"source":"10.0.1.20","destination":"10.0.1.10","protocol":"tcp"}}}]},
            {"disposition":"blocked","forward":{"disposition":"blocked","hops":[],"final_flow":{}},"returns":[]}
        ]});
        sample["branch_index"]=json!(0); sample["return_index"]=json!(0);
        let out=project_topology(sample.clone()).unwrap();
        assert_eq!(out["topology"]["traffic"]["selected_return"]["session_status"],"established");
        sample["branch_index"]=json!(2);
        assert!(project_topology(sample).is_err());
    }
}
