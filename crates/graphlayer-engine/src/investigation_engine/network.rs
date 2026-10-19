use serde_json::{json, Value};
use std::{collections::{HashMap, HashSet}, net::Ipv4Addr, str::FromStr};
use super::path::{evaluate_path_inner, evaluate_return_with_forward};
use sha2::{Digest, Sha256};

fn s<'a>(v: &'a Value, k: &str) -> &'a str { v.get(k).and_then(Value::as_str).unwrap_or("") }
fn flow_json(v:&Value)->Value{let mut f=v.clone();if let Some(o)=f.as_object_mut(){for k in ["source_port","destination_port"]{if o.get(k).and_then(Value::as_i64).unwrap_or(0)==0{o.remove(k);}}}f}
fn zero_flow()->Value{json!({"scope":{"snapshot_id":"","device_id":"","context":""},"source":"","destination":"","protocol":""})}
fn arr(v: &Value, k: &str) -> Vec<Value> { v.get(k).and_then(Value::as_array).cloned().unwrap_or_default() }
fn b(v: &Value, k: &str) -> bool { v.get(k).and_then(Value::as_bool).unwrap_or(false) }
fn ip(v: &str) -> Option<u32> { Ipv4Addr::from_str(v).ok().map(u32::from) }
fn prefix(v: &str) -> Option<(u32, u8)> {
    let (addr, bits) = v.split_once('/')?;
    let bits = bits.parse::<u8>().ok()?;
    if bits > 32 { return None; }
    let addr = ip(addr)?;
    let mask = if bits == 0 { 0 } else { u32::MAX << (32 - bits) };
    Some((addr & mask, bits))
}
fn contains(net: u32, bits: u8, addr: u32) -> bool {
    let mask = if bits == 0 { 0 } else { u32::MAX << (32 - bits) };
    net & mask == addr & mask
}
fn iface(node: &Value, name: &str) -> Option<Value> {
    let matches: Vec<_> = arr(&node["config"]["path"], "interfaces").into_iter()
        .filter(|i| s(i, "name").eq_ignore_ascii_case(name)).collect();
    if matches.len() == 1 { Some(matches[0].clone()) } else { None }
}
fn usable_host(addr: u32, net: u32, bits: u8) -> bool {
    if !contains(net, bits, addr) || addr == 0 || addr == u32::MAX || addr >> 28 == 0xe { return false; }
    if bits >= 31 { return true; }
    let host = (1u32 << (32 - bits)) - 1;
    addr != net && addr != net | host
}
fn connected(node: &Value, name: &str, addr: &str) -> bool {
    let Some(i) = iface(node, name) else { return false; };
    let Some((net, bits)) = prefix(s(&i, "address")) else { return false; };
    let Some(addr) = ip(addr) else { return false; };
    b(&i, "resolved") && !b(&i, "shutdown") && usable_host(addr, net, bits)
        && ip(s(&i, "address").split('/').next().unwrap_or("")) != Some(addr)
        && arr(&node["config"]["path"], "routes").iter().any(|r|
            s(r, "kind").eq_ignore_ascii_case("connected")
                && s(r, "interface").eq_ignore_ascii_case(name)
                && prefix(s(r, "prefix")) == Some((net, bits)))
}
fn endpoint_eq(e: &Value, node: &str, interface: &str) -> bool {
    s(e, "node_id") == node && s(e, "interface").eq_ignore_ascii_case(interface)
}
fn refs(snapshot: &str, refs: Vec<String>) -> Vec<Value> {
    refs.into_iter().map(|r| json!({"snapshot_id": snapshot, "ref": r})).collect()
}
fn link_refs(nodes: &HashMap<String, Value>, link: &Value) -> Vec<Value> {
    let mut out = Vec::new();
    for e in [&link["a"], &link["b"]] {
        if let Some(n) = nodes.get(s(e, "node_id")) {
            if let Some(i) = iface(n, s(e, "interface")) {
                out.extend(refs(s(&n["scope"], "snapshot_id"), arr(&i, "evidence_refs").iter().filter_map(Value::as_str).map(str::to_owned).collect()));
            }
        }
    }
    out
}
fn path_input(node: &Value, ingress: &str, flow: &Value) -> Value {
    json!({"scope": node["scope"], "config": node["config"], "ingress_interface": ingress, "flow": flow})
}
fn fresh_result(input: &Value, vendor: bool) -> Value {
    json!({"engine_contract_version":if vendor{"graphlayer-network-path-v2"}else{"graphlayer-network-path-v1"},"original_flow":flow_json(&input["flow"]),"forward":{"disposition":"indeterminate","hops":[],"final_flow":zero_flow(),"reasons":[]},"return":{"disposition":"not_evaluated","hops":[],"final_flow":zero_flow(),"reasons":[]},"disposition":"indeterminate","reasons":[],"assumptions":["configuration and topology are supplied evidence, not live device or link state","only static IPv4 forwarding supported by the single-device path model is evaluated","a modeled return leg assumes the reverse TCP/UDP tuple belongs to the established session represented by the forward permit; live connection state is not queried"]})
}
fn uncertain(mut result: Value, reason: &str) -> Value {
    result["forward"]["reasons"].as_array_mut().unwrap().push(json!(reason));
    result["reasons"].as_array_mut().unwrap().push(json!(reason));
    result
}

#[derive(serde::Serialize)]
struct BranchIdentity<'a> { #[serde(rename="Node")] node: &'a str, #[serde(rename="Candidate")] candidate: &'a str, #[serde(rename="Route")] route: &'a str, #[serde(rename="NAT")] nat: Vec<String> }
fn branch_id(forward: &Value, candidates: &[String]) -> String {
    let hops=arr(&forward,"hops");let rows:Vec<_>=hops.iter().enumerate().map(|(i,h)|BranchIdentity{node:s(h,"node_id"),candidate:candidates.get(i).map(String::as_str).unwrap_or(""),route:s(&h["path"]["selected_route"],"id"),nat:arr(&h["path"],"nat_rule_ids").iter().filter_map(Value::as_str).map(str::to_owned).collect()}).collect();
    let bytes=serde_json::to_vec(&rows).unwrap_or_default();let digest=Sha256::digest(bytes);format!("branch-{}",digest.iter().map(|b|format!("{b:02x}")).collect::<String>())
}
fn exploration_branch(result:&Value,candidates:&[String])->Value {
    let id=branch_id(&result["forward"],candidates);let rd=s(&result["return"],"disposition");let returns=if rd=="not_evaluated"{json!([])}else{let status=if rd=="modeled_forward"{"established"}else{"unproven"};let disposition=if rd=="modeled_forward"&&s(result,"disposition")=="modeled_round_trip"{"modeled_forward"}else if rd=="blocked"{"blocked"}else{"indeterminate"};json!([{"id":format!("{id}-return"),"direction":result["return"],"asymmetric":disposition!="modeled_forward"&&rd=="modeled_forward","session_status":status,"disposition":disposition,"reasons":result["return"]["reasons"]}])};
    json!({"id":id,"forward":result["forward"],"returns":returns,"disposition":result["disposition"],"reasons":result["reasons"]})
}

/// Compose bounded single-firewall evaluations over the explicitly supplied graph.
pub fn evaluate_network(input: Value) -> Result<Value, String> {
    evaluate_network_forced(input, &HashMap::new())
}

fn evaluate_network_forced(input: Value, forced: &HashMap<String, String>) -> Result<Value, String> {
    let nodes = arr(&input, "nodes");
    let links = arr(&input, "links");
    let vendor = nodes.iter().any(|n| { let v=s(&n["config"],"vendor").trim().to_ascii_lowercase(); !v.is_empty() && !["cisco-asa","cisco_asa","asa"].contains(&v.as_str()) });
    let mut result = fresh_result(&input, vendor);
    if nodes.is_empty() || nodes.len() > 16 || links.len() > 32 { return Ok(uncertain(result, "topology exceeds supported node or link limits")); }
    let source = &input["source"];
    let destination = &input["destination"];
    if s(source,"node_id").is_empty() || s(source,"interface").is_empty() || s(destination,"node_id").is_empty() || s(destination,"interface").is_empty()
        || s(source,"node_id").len()>64 || s(destination,"node_id").len()>64 || s(source,"interface").len()>48 || s(destination,"interface").len()>48 {
        return Err("source and destination node/interface are required".into());
    }
    let mut by_id = HashMap::<String, Value>::new();
    for node in &nodes {
        let id = s(node,"id");
        if id.is_empty() || id.len()>64 || by_id.insert(id.to_owned(),node.clone()).is_some() {
            return Ok(uncertain(result,"topology contains a missing, overlong, or duplicate node identifier"));
        }
    }
    let Some(start) = by_id.get(s(source,"node_id")) else { return Ok(uncertain(result,"source node is absent from topology")); };
    if !by_id.contains_key(s(destination,"node_id")) { return Ok(uncertain(result,"destination node is absent from topology")); }
    let mut used = HashSet::new();
    for link in &links {
        if s(&link["a"],"node_id").is_empty() || s(&link["b"],"node_id").is_empty()
            || s(&link["a"],"interface").is_empty() || s(&link["b"],"interface").is_empty()
            || s(&link["a"],"node_id") == s(&link["b"],"node_id") { return Ok(uncertain(result,"topology contains an invalid link endpoint")); }
        for e in [&link["a"],&link["b"]] {
            let id=s(e,"node_id"); let name=s(e,"interface");
            let Some(node)=by_id.get(id) else { return Ok(uncertain(result,"link references a missing node")); };
            if id.len()>64 || name.len()>48 { return Ok(uncertain(result,"topology identifier exceeds supported length")); }
            if !used.insert(format!("{}\0{}",id,name.to_lowercase())) { return Ok(uncertain(result,"interface endpoint is repeated in topology links")); }
            let Some(i)=iface(node,name) else { return Ok(uncertain(result,"link endpoint interfaces are unresolved or do not share one IPv4 subnet")); };
            if !b(&i,"resolved") || b(&i,"shutdown") { return Ok(uncertain(result,"link endpoint interfaces are unresolved or do not share one IPv4 subnet")); }
        }
        let ia=iface(by_id.get(s(&link["a"],"node_id")).unwrap(),s(&link["a"],"interface")).unwrap();
        let ib=iface(by_id.get(s(&link["b"],"node_id")).unwrap(),s(&link["b"],"interface")).unwrap();
        let pa=prefix(s(&ia,"address")); let pb=prefix(s(&ib,"address"));
        let aa=ip(s(&ia,"address").split('/').next().unwrap_or("")); let ab=ip(s(&ib,"address").split('/').next().unwrap_or(""));
        if pa.is_none() || pa!=pb || aa==ab || !aa.zip(pa).is_some_and(|(x,(n,bits))|usable_host(x,n,bits)) || !ab.zip(pb).is_some_and(|(x,(n,bits))|usable_host(x,n,bits)) {
            return Ok(uncertain(result,"link endpoint interfaces are unresolved or do not share one IPv4 subnet"));
        }
    }
    if input["flow"]["scope"] != start["scope"] { return Err("flow scope must match the source node scope".into()); }
    let mut current=source.clone(); let mut flow=input["flow"].clone(); let mut visited=HashSet::new();
    for hop in 0..16 {
        let id=s(&current,"node_id").to_owned();
        if !visited.insert(id.clone()) { return Ok(uncertain(result,"topology traversal contains a cycle")); }
        let node=by_id.get(&id).unwrap();
        if hop==0 && !connected(node,s(&current,"interface"),s(&flow,"source")) { return Ok(uncertain(result,"source address is not proven on the selected ingress interface subnet")); }
        flow["scope"]=node["scope"].clone();
        let route_id = forced.get(&id).map(String::as_str);
        let path=match evaluate_path_inner(path_input(node,s(&current,"interface"),&flow),None,route_id) { Ok(x)=>x,Err(e)=>return Ok(uncertain(result,&format!("node path inputs could not be evaluated: {e}"))) };
        let snap=s(&node["scope"],"snapshot_id");
        let ev=refs(snap,arr(&path,"evidence_refs").iter().filter_map(Value::as_str).map(str::to_owned).collect());
        let mut hop_value=json!({"node_id":id,"snapshot_id":snap,"path":path});if !ev.is_empty(){hop_value["evidence"]=json!(ev)}result["forward"]["hops"].as_array_mut().unwrap().push(hop_value);
        let last=result["forward"]["hops"].as_array_mut().unwrap().len()-1;
        if s(&path,"disposition")!="modeled_forward" {
            result["forward"]["disposition"]=json!(s(&path,"disposition")); result["forward"]["reasons"]=path["reasons"].clone(); result["reasons"]=path["reasons"].clone();
            if s(&path,"disposition")=="blocked" { result["disposition"]=json!("blocked"); }
            return Ok(result);
        }
        flow=path["egress_flow"].clone();
        if id==s(destination,"node_id") {
            if !s(&path,"egress_interface").eq_ignore_ascii_case(s(destination,"interface")) { return Ok(uncertain(result,"destination node egress differs from requested terminal interface")); }
            if !connected(node,s(destination,"interface"),s(&flow,"destination")) { return Ok(uncertain(result,"terminal destination is not proven directly connected on requested egress interface")); }
            result["forward"]["disposition"]=json!("modeled_forward"); result["forward"]["final_flow"]=flow.clone();
            let (returns,ret_disposition,ret_reasons,final_flow)=evaluate_return_path(&input,&by_id,&links,result["forward"]["hops"].as_array().unwrap())?;
            result["return"]["hops"]=json!(returns); result["return"]["disposition"]=json!(ret_disposition);
            if !ret_reasons.is_empty() { result["return"]["reasons"]=json!(ret_reasons); result["reasons"]=json!(ret_reasons); result["disposition"]=json!(if ret_disposition=="blocked"{"blocked"}else{"indeterminate"}); }
            else { result["return"]["final_flow"]=final_flow; result["disposition"]=json!("modeled_round_trip"); }
            return Ok(result);
        }
        let mut selected=None; let mut peer=None;
        for link in &links {
            if endpoint_eq(&link["a"],&id,s(&path,"egress_interface")) { selected=Some(link.clone());peer=Some(link["b"].clone());break; }
            if endpoint_eq(&link["b"],&id,s(&path,"egress_interface")) { selected=Some(link.clone());peer=Some(link["a"].clone());break; }
        }
        let Some(link)=selected else { return Ok(uncertain(result,"selected egress has no explicit topology link")); };
        let peer=peer.unwrap(); let peer_node=by_id.get(s(&peer,"node_id")).unwrap(); let route=&path["selected_route"];
        let (Some(pi),Some(li))=(iface(peer_node,s(&peer,"interface")),iface(node,s(&path,"egress_interface"))) else { return Ok(uncertain(result,"selected route does not prove the linked peer interface as its next hop")); };
        let peer_prefix=prefix(s(&pi,"address")); let local_prefix=prefix(s(&li,"address"));
        let peer_addr=ip(s(&pi,"address").split('/').next().unwrap_or(""));
        if !s(route,"kind").eq_ignore_ascii_case("static") || ip(s(route,"next_hop"))!=peer_addr || peer_prefix!=local_prefix {
            return Ok(uncertain(result,"selected route does not prove the linked peer interface as its next hop"));
        }
        result["forward"]["hops"][last]["link"]=link.clone();
        result["forward"]["hops"][last]["evidence"].as_array_mut().unwrap().extend(link_refs(&by_id,&link));
        current=peer;
    }
    Ok(uncertain(result,"topology traversal exceeds supported hop limit"))
}

fn evaluate_return_path(input:&Value,nodes:&HashMap<String,Value>,links:&[Value],forward_hops:&[Value])->Result<(Vec<Value>,String,Vec<String>,Value),String>{
 let mut hop_results=Vec::new(); let mut carried:Option<Value>=None;
 for index in (0..forward_hops.len()).rev(){
  let h=&forward_hops[index];let node=nodes.get(s(h,"node_id")).unwrap();let mut flow=h["path"]["egress_flow"].clone();
  let source=flow["source"].clone();flow["source"]=flow["destination"].clone();flow["destination"]=source;let source_port=flow["source_port"].as_i64().unwrap_or(0);let destination_port=flow["destination_port"].as_i64().unwrap_or(0);if destination_port==0{flow.as_object_mut().unwrap().remove("source_port");}else{flow["source_port"]=json!(destination_port)}if source_port==0{flow.as_object_mut().unwrap().remove("destination_port");}else{flow["destination_port"]=json!(source_port)}flow["scope"]=node["scope"].clone();
  if let Some(next)=carried.take(){for k in ["source","destination","protocol","source_port","destination_port"]{if next[k]!=flow[k]{return Ok((hop_results,"indeterminate".into(),vec!["return tuple from the next firewall does not match this firewall's expected reverse tuple".into()],Value::Null))}}flow=next;}
  let path=evaluate_return_with_forward(path_input(node,s(&h["path"],"egress_interface"),&flow),h["path"].clone()).map_err(|e|format!("return path is not established on node {}: {e}",s(h,"node_id")))?;
  let snap=s(&node["scope"],"snapshot_id");let ev=refs(snap,arr(&path,"evidence_refs").iter().filter_map(Value::as_str).map(str::to_owned).collect());
  let mut hop_value=json!({"node_id":s(h,"node_id"),"snapshot_id":snap,"path":path});if !ev.is_empty(){hop_value["evidence"]=json!(ev)}hop_results.push(hop_value);
  if s(&path,"disposition")!="modeled_forward" {let disp=s(&path,"disposition").to_owned();return Ok((hop_results,disp,vec![format!("return path is blocked or indeterminate on node {}",s(h,"node_id"))],Value::Null));}
  if index==0 {
    if !connected(node,s(&input["source"],"interface"),s(&path["egress_flow"],"destination")) {return Ok((hop_results,"indeterminate".into(),vec!["return destination is not proven on the requested source interface subnet".into()],Value::Null));}
    return Ok((hop_results,"modeled_forward".into(),vec![],path["egress_flow"].clone()));
  }
  let prev=&forward_hops[index-1]; let mut reverse_link=None;
  for link in links {
   let forward=(endpoint_eq(&link["a"],s(h,"node_id"),s(&path,"egress_interface"))&&endpoint_eq(&link["b"],s(prev,"node_id"),s(&prev["path"],"egress_interface")))
    ||(endpoint_eq(&link["b"],s(h,"node_id"),s(&path,"egress_interface"))&&endpoint_eq(&link["a"],s(prev,"node_id"),s(&prev["path"],"egress_interface")));
   if forward {reverse_link=Some(link.clone());break;}
  }
  let Some(link)=reverse_link else{return Ok((hop_results,"indeterminate".into(),vec!["return egress does not use the explicit reverse topology link".into()],Value::Null));};
  let peer=if s(&link["a"],"node_id")==s(h,"node_id"){&link["b"]}else{&link["a"]};let peer_node=nodes.get(s(prev,"node_id")).unwrap();
  let (Some(local),Some(peer_i))=(iface(node,s(&path,"egress_interface")),iface(peer_node,s(peer,"interface")))else{return Ok((hop_results,"indeterminate".into(),vec!["return route does not prove the linked peer interface as its next hop".into()],Value::Null));};
  let route=&path["selected_route"];let peer_addr=ip(s(&peer_i,"address").split('/').next().unwrap_or(""));
  if !s(route,"kind").eq_ignore_ascii_case("static")||ip(s(route,"next_hop"))!=peer_addr||prefix(s(&local,"address"))!=prefix(s(&peer_i,"address")){return Ok((hop_results,"indeterminate".into(),vec!["return route does not prove the linked peer interface as its next hop".into()],Value::Null));}
  let last=hop_results.len()-1;hop_results[last]["link"]=link.clone();hop_results[last]["evidence"].as_array_mut().unwrap().extend(link_refs(nodes,&link));carried=Some(path["egress_flow"].clone());
 }
 Ok((hop_results,"indeterminate".into(),vec!["return path did not reach its source".into()],Value::Null))
}

#[allow(dead_code)]
fn explore_network_legacy(input:Value)->Result<Value,String>{
 let base=evaluate_network(input.clone())?;let mut base_ids=Vec::new();for hop in arr(&base["forward"],"hops"){let node=arr(&input,"nodes").into_iter().find(|n|s(n,"id")==s(&hop,"node_id"));if let Some(node)=node{let pi=path_input(&node,s(&hop["path"],"ingress_interface"),&hop["path"]["original_flow"]);let cs=super::path::explore_path(pi)?;let matched=arr(&cs,"candidates").into_iter().find(|c|c["result"]==hop["path"]);base_ids.push(matched.map(|c|s(&c,"id").to_owned()).unwrap_or_default());}else{base_ids.push(String::new())}}
 let mut branches=vec![exploration_branch(&base,&base_ids)];let mut truncated=false;let mut evaluations=1usize;let mut trunc_reasons=Vec::<Value>::new();
 for hop in arr(&base["forward"],"hops") {
  let node_id=s(&hop,"node_id").to_owned();let node=arr(&input,"nodes").into_iter().find(|n|s(n,"id")==node_id);let Some(node)=node else{continue};
  let node_input=path_input(&node,s(&hop["path"],"ingress_interface"),&hop["path"]["original_flow"]);let alternatives=super::path::explore_path(node_input)?;
  for candidate in arr(&alternatives,"candidates") {
   if s(&candidate["result"],"disposition")!="modeled_forward"||candidate["result"]["selected_route"]==hop["path"]["selected_route"]{continue}
   if branches.len()>=32 {truncated=true;trunc_reasons.push(json!("terminal branch limit reached"));break}
   if evaluations>=128 {truncated=true;trunc_reasons.push(json!("hop evaluation limit reached"));break}
   let route=s(&candidate["result"]["selected_route"],"id");if route.is_empty(){continue}let vendor=s(&node["config"],"vendor").to_ascii_lowercase();let forced_route=if vendor.contains("pan"){let id=s(&candidate,"id").strip_prefix("candidate:").unwrap_or("");let mut parts=id.split('/');let pre=parts.next().unwrap_or(route);let post=parts.next().unwrap_or(pre);format!("pre={pre};post={post}")}else{route.to_owned()};let mut forced=HashMap::new();forced.insert(node_id.clone(),forced_route);let result=evaluate_network_forced(input.clone(),&forced)?;evaluations+=arr(&result["forward"],"hops").len().max(1);
   let mut ids=Vec::new();for h in arr(&result["forward"],"hops"){let n=arr(&input,"nodes").into_iter().find(|n|s(n,"id")==s(&h,"node_id"));let Some(n)=n else{ids.push(String::new());continue};let pi=path_input(&n,s(&h["path"],"ingress_interface"),&h["path"]["original_flow"]);let cs=super::path::explore_path(pi)?;let matching=arr(&cs,"candidates").into_iter().find(|c|c["result"]==h["path"]);ids.push(matching.map(|c|s(&c,"id").to_owned()).unwrap_or_default());}
   branches.push(exploration_branch(&result,&ids));
  }
  if truncated {break}
 }
 branches.sort_by(|a,b|s(a,"id").cmp(s(b,"id")));let all_blocked=!branches.is_empty()&&branches.iter().all(|b|s(b,"disposition")=="blocked");let all_round=!branches.is_empty()&&branches.iter().all(|b|s(b,"disposition")=="modeled_round_trip");let disposition=if all_blocked&&!truncated{"blocked"}else if all_round&&!truncated{"modeled_round_trip"}else{"indeterminate"};let complete=!truncated;let mut reasons=Vec::new();for r in trunc_reasons{if !reasons.contains(&r){reasons.push(r)}}
 Ok(json!({"engine_contract_version":"graphlayer-network-exploration-v1","original_flow":flow_json(&input["flow"]),"branches":branches,"disposition":disposition,"complete":complete,"truncated":truncated,"reasons":reasons,"assumptions":["configuration and topology are supplied evidence, not live device or link state","only explicitly linked IPv4 paths and bounded route choices are explored","equal-best route choices are configured alternatives; this does not prove ECMP is enabled or predict a hardware hash/session choice","return session state is modeled only when the exact reverse tuple and selected forward state are supported"]}))
}
