use serde_json::{json, Value};
use sha2::{Digest, Sha256};
use std::collections::{HashMap, HashSet};
use std::net::Ipv4Addr;
use std::str::FromStr;

const MAX_BRANCHES: usize = 32;
const MAX_EVALUATIONS: usize = 128;
const MAX_HOPS: usize = 16;

fn s<'a>(v: &'a Value, k: &str) -> &'a str { v.get(k).and_then(Value::as_str).unwrap_or("") }
fn b(v: &Value, k: &str) -> bool { v.get(k).and_then(Value::as_bool).unwrap_or(false) }
fn a(v: &Value, k: &str) -> Vec<Value> { v.get(k).and_then(Value::as_array).cloned().unwrap_or_default() }
fn scope(v: &Value) -> Value { json!({"snapshot_id":s(v,"snapshot_id"),"device_id":s(v,"device_id"),"context":s(v,"context")}) }
fn endpoint(node: &str, iface: &str) -> Value { json!({"node_id":node,"interface":iface}) }
fn link_json(v:&Value)->Value { json!({"a":endpoint(s(&v["a"],"node_id"),s(&v["a"],"interface")),"b":endpoint(s(&v["b"],"node_id"),s(&v["b"],"interface"))}) }
fn flow_tuple(v: &Value) -> Value {
    let mut f=json!({"scope":scope(&v["scope"]),"source":s(v,"source"),"destination":s(v,"destination"),"protocol":s(v,"protocol")});
    for k in ["source_port","destination_port"] { if let Some(n)=v.get(k).and_then(Value::as_i64).filter(|n|*n!=0) { f[k]=json!(n); } }
    f
}
fn reverse_flow(v:&Value, sc:&Value)->Value {
    let mut out=flow_tuple(v);out["scope"]=scope(sc);let src=out["source"].clone();out["source"]=out["destination"].clone();out["destination"]=src;
    let sp=out["source_port"].clone();let dp=out["destination_port"].clone();
    if dp.is_null(){out.as_object_mut().unwrap().remove("source_port");}else{out["source_port"]=dp;}
    if sp.is_null(){out.as_object_mut().unwrap().remove("destination_port");}else{out["destination_port"]=sp;}
    out
}
fn iface<'a>(node:&'a Value,name:&str)->Option<&'a Value>{let xs=node["config"]["path"]["interfaces"].as_array()?;let mut found=xs.iter().filter(|x|s(x,"name").eq_ignore_ascii_case(name));let first=found.next()?;if found.next().is_some(){None}else{Some(first)}}
fn ip(v:&str)->Option<u32>{Ipv4Addr::from_str(v).ok().map(u32::from)}
fn prefix(v:&str)->Option<(u32,u8)>{let (addr,bits)=v.split_once('/')?;let bits=bits.parse::<u8>().ok()?;if bits>32{return None}let addr=ip(addr)?;let mask=if bits==0{0}else{u32::MAX<<(32-bits)};Some((addr&mask,bits))}
fn contains(net:u32,bits:u8,addr:u32)->bool{let mask=if bits==0{0}else{u32::MAX<<(32-bits)};net&mask==addr&mask}
fn usable(addr:u32,net:u32,bits:u8)->bool{if !contains(net,bits,addr)||addr==0||addr==u32::MAX||addr>>28==0xe{return false}if bits>=31{return true}let host=(1u32<<(32-bits))-1;addr!=net&&addr!=net|host}
fn connected(node:&Value,name:&str,addr:&str)->bool{let Some(i)=iface(node,name)else{return false};let Some((net,bits))=prefix(s(i,"address"))else{return false};let Some(addr)=ip(addr)else{return false};b(i,"resolved")&&!b(i,"shutdown")&&usable(addr,net,bits)&&ip(s(i,"address").split('/').next().unwrap_or(""))!=Some(addr)&&a(&node["config"]["path"],"routes").iter().any(|r|s(r,"kind").eq_ignore_ascii_case("connected")&&s(r,"interface").eq_ignore_ascii_case(name)&&prefix(s(r,"prefix"))==Some((net,bits)))}
fn link_facts(nodes:&HashMap<String,Value>,link:&Value)->bool{let(Some(na),Some(nb))=(nodes.get(s(&link["a"],"node_id")),nodes.get(s(&link["b"],"node_id")))else{return false};let(Some(ia),Some(ib))=(iface(na,s(&link["a"],"interface")),iface(nb,s(&link["b"],"interface")))else{return false};let(pa,pb)=(prefix(s(ia,"address")),prefix(s(ib,"address")));let(aa,ab)=(ip(s(ia,"address").split('/').next().unwrap_or("")),ip(s(ib,"address").split('/').next().unwrap_or("")));b(ia,"resolved")&&!b(ia,"shutdown")&&b(ib,"resolved")&&!b(ib,"shutdown")&&pa.is_some()&&pa==pb&&aa!=ab&&aa.zip(pa).is_some_and(|(x,(n,b))|usable(x,n,b))&&ab.zip(pb).is_some_and(|(x,(n,b))|usable(x,n,b))}
fn route_reaches(node:&Value,peer:&Value,path:&Value,peer_name:&str)->bool{
 let Some(route)=path.get("selected_route")else{return false};let(Some(pi),Some(li))=(iface(peer,peer_name),iface(node,s(path,"egress_interface")))else{return false};
 let pa=ip(s(pi,"address").split('/').next().unwrap_or(""));let nh=ip(s(route,"next_hop"));
 s(route,"kind").eq_ignore_ascii_case("static")&&s(route,"interface").eq_ignore_ascii_case(s(path,"egress_interface"))&&pa.is_some()&&pa==nh&&prefix(s(pi,"address"))==prefix(s(li,"address"))&&b(pi,"resolved")&&!b(pi,"shutdown")&&b(li,"resolved")&&!b(li,"shutdown")
}
fn refs(snapshot:&str,values:Vec<String>)->Vec<Value>{values.into_iter().filter(|x|!x.is_empty()).map(|r|json!({"snapshot_id":snapshot,"ref":r})).collect()}
fn link_refs(nodes:&HashMap<String,Value>,link:&Value)->Vec<Value>{let mut out=Vec::new();for e in [&link["a"],&link["b"]]{if let Some(n)=nodes.get(s(e,"node_id")){if let Some(i)=iface(n,s(e,"interface")){out.extend(refs(s(&n["scope"],"snapshot_id"),a(i,"evidence_refs").iter().filter_map(Value::as_str).map(str::to_owned).collect()));}}}out}
fn branch_id(hops:&[Value],ids:&[String])->String{
 #[derive(serde::Serialize)]struct Identity<'a>{#[serde(rename="Node")]node:&'a str,#[serde(rename="Candidate")]candidate:&'a str,#[serde(rename="Route")]route:&'a str,#[serde(rename="NAT")]nat:Vec<String>}
 let rows= hops.iter().enumerate().map(|(i,h)|Identity{node:s(h,"node_id"),candidate:ids.get(i).map(String::as_str).unwrap_or(""),route:s(&h["path"]["selected_route"],"id"),nat:a(&h["path"],"nat_rule_ids").iter().filter_map(Value::as_str).map(str::to_owned).collect()}).collect::<Vec<_>>();
 let bytes=serde_json::to_vec(&rows).unwrap_or_default();format!("branch-{}",Sha256::digest(bytes).iter().map(|x|format!("{x:02x}")).collect::<String>())
}

#[derive(Clone)]
struct State { current:Value, flow:Value, hops:Vec<Value>, visited:HashSet<String>, forward_by_node:HashMap<String,Value>, candidate_ids:Vec<String>, session:String, asymmetric:bool, disposition:String, reasons:Vec<String> }
struct Ctx { input:Value, nodes:HashMap<String,Value>, links:Vec<Value>, evaluations:usize, terminal:usize, truncated:bool, complete:bool, reasons:Vec<String> }
impl Ctx {
 fn truncate(&mut self,reason:&str){self.truncated=true;self.complete=false;if !self.reasons.iter().any(|r|r==reason){self.reasons.push(reason.to_owned())}}
 fn finish(&mut self,mut st:State,returning:bool,disp:&str,reason:&str)->Value{
  self.terminal+=1;let reason=if reason.is_empty(){disp}else{reason};let id=branch_id(&st.hops,&st.candidate_ids);
  if !returning {st.disposition=disp.to_owned();if !reason.is_empty(){st.reasons.push(reason.to_owned())}let final_flow=if disp=="modeled_forward"{flow_tuple(&st.flow)}else{zero_flow()};return json!({"id":id,"forward":{"disposition":st.disposition,"hops":st.hops,"final_flow":final_flow,"reasons":st.reasons},"returns":[],"disposition":disp,"reasons":[reason]})}
  let final_flow=if disp=="modeled_forward"{flow_tuple(&st.flow)}else{zero_flow()};
  let direction=json!({"disposition":disp,"hops":st.hops,"final_flow":final_flow,"reasons":st.reasons});
  let mut bd=disp.to_owned();let mut why=reason.to_owned();let status=if st.session.is_empty(){"unproven"}else{&st.session};
  if bd=="modeled_forward"&&status!="established"{bd="indeterminate".into();if why=="modeled_forward"||why.is_empty(){why="configured return route is concrete, but established session state is unproven".into();}}
  json!({"id":id,"forward":{"disposition":"indeterminate","hops":[],"final_flow":zero_flow(),"reasons":[]},"returns":[{"id":format!("{id}-return"),"direction":direction,"asymmetric":st.asymmetric,"session_status":status,"disposition":bd,"reasons":[why]}],"disposition":"indeterminate","reasons":[why]})
 }
 fn walk(&mut self,mut st:State,returning:bool)->Vec<Value>{
  if self.terminal>=MAX_BRANCHES{self.truncate("terminal branch limit reached");return vec![]}
  if st.hops.len()>=MAX_HOPS||st.visited.len()>=MAX_HOPS{return vec![self.finish(st,returning,"indeterminate","path exceeds supported hop limit")]}
  let node_id=s(&st.current,"node_id").to_owned();if st.visited.contains(&node_id){return vec![self.finish(st,returning,"indeterminate","topology traversal contains a cycle")]}
  let Some(node)=self.nodes.get(&node_id).cloned()else{return vec![self.finish(st,returning,"indeterminate","selected link references a missing node")]};
  st.visited.insert(node_id.clone());st.flow["scope"]=scope(&node["scope"]);
  if self.evaluations>=MAX_EVALUATIONS{self.truncate("hop evaluation limit reached");return vec![]}
  let pin=json!({"scope":node["scope"],"config":node["config"],"ingress_interface":st.current["interface"],"flow":st.flow.clone()});
  let candidates=if returning{let mut ri=pin;if let Some(fwd)=st.forward_by_node.get(&node_id){ri["forward"]=fwd.clone()}else{ri["forward"]=Value::Null}super::path::explore_return(ri)}else{super::path::explore_path(pin)};
  let candidates=match candidates{Ok(x)=>x,Err(e)=>return vec![self.finish(st,returning,"indeterminate",&format!("node path inputs could not be evaluated: {e}"))]};
  if b(&candidates,"truncated"){self.truncate("node route choice limit reached")}
  let mut choices=a(&candidates,"candidates");if choices.is_empty(){return vec![self.finish(st,returning,"indeterminate","no supported route candidate was produced")]}
  let remaining=MAX_EVALUATIONS.saturating_sub(self.evaluations);if choices.len()>remaining{choices.truncate(remaining);self.truncate("hop evaluation limit reached")}
  self.evaluations+=choices.len();let mut out=Vec::new();
  for c in choices{
   if self.terminal>=MAX_BRANCHES{self.truncate("terminal branch limit reached");break}
   let mut branch=st.clone();let cid=s(&c,"id").to_owned();branch.candidate_ids.push(cid.clone());let mut p=c["result"].clone();let resolved=b(&c,"resolved");
   if !resolved&&s(&p,"disposition")=="modeled_forward"{p["disposition"]=json!("indeterminate");p["reasons"].as_array_mut().unwrap().push(json!("candidate egress tuple is not fully resolved"));}
   if returning{branch.session=merge_session(&branch.session,s(&c,"session_status"));if let Some(fwd)=branch.forward_by_node.get(&node_id){if !s(fwd,"egress_interface").eq_ignore_ascii_case(s(&branch.current,"interface")){branch.asymmetric=true;branch.session=merge_session(&branch.session,"asymmetric_unknown")}else if branch.session=="asymmetric_unknown"{branch.asymmetric=true}}else{branch.asymmetric=true;branch.session=merge_session(&branch.session,"unproven")}}
   let evidence=refs(s(&node["scope"],"snapshot_id"),a(&p,"evidence_refs").iter().filter_map(Value::as_str).map(str::to_owned).collect());let mut h=json!({"node_id":node_id.clone(),"snapshot_id":s(&node["scope"],"snapshot_id"),"path":p.clone()});if !evidence.is_empty(){h["evidence"]=json!(evidence)}branch.hops.push(h);
   let concrete=returning&&resolved&&!s(&p,"egress_interface").is_empty()&&!s(&p["egress_flow"],"source").is_empty()&&!s(&p["egress_flow"],"destination").is_empty();
   if s(&p,"disposition")!="modeled_forward"&&!concrete{out.push(self.finish(branch,returning,s(&p,"disposition"),&a(&p,"reasons").iter().filter_map(Value::as_str).collect::<Vec<_>>().join("; ")));continue}
   branch.flow=p["egress_flow"].clone();if !returning{branch.forward_by_node.insert(node_id.clone(),p.clone());}
   if returning{
    if node_id==s(&self.input["source"],"node_id")&&s(&p,"egress_interface").eq_ignore_ascii_case(s(&self.input["source"],"interface"))&&connected(&node,s(&self.input["source"],"interface"),s(&p["egress_flow"],"destination")){out.push(self.finish(branch,true,"modeled_forward",""));}
    else{out.extend(self.advance(branch,true,&p));}continue
   }
   if node_id==s(&self.input["destination"],"node_id"){
    if s(&p,"egress_interface").eq_ignore_ascii_case(s(&self.input["destination"],"interface"))&&connected(&node,s(&self.input["destination"],"interface"),s(&p["egress_flow"],"destination")){
     let forward=branch.clone();let sc=scope(&self.nodes[s(&self.input["destination"],"node_id")]["scope"]);let rf=reverse_flow(&p["egress_flow"],&sc);
     let ret=State{current:self.input["destination"].clone(),flow:rf,hops:vec![],visited:HashSet::new(),forward_by_node:branch.forward_by_node.clone(),candidate_ids:vec![],session:String::new(),asymmetric:false,disposition:"indeterminate".into(),reasons:vec![]};
     let returns=self.walk(ret,true);out.push(self.combine_returns(forward,returns));
    }else{out.push(self.finish(branch,false,"indeterminate","destination node egress differs from requested directly connected terminal"));}
    continue
   }
   out.extend(self.advance(branch,false,&p));
  }
  out
 }
 fn advance(&mut self,mut st:State,returning:bool,p:&Value)->Vec<Value>{
  let current=s(&st.current,"node_id").to_owned();let egress=s(p,"egress_interface");let mut peers=Vec::<(Value,Value)>::new();let mut endpoint_found=false;
  for link in &self.links {for (local,peer) in [(&link["a"],&link["b"]),(&link["b"],&link["a"])]{if s(local,"node_id")==current&&s(local,"interface").eq_ignore_ascii_case(egress){endpoint_found=true;if let(Some(n),Some(pn))=(self.nodes.get(&current),self.nodes.get(s(peer,"node_id"))){if route_reaches(n,pn,p,s(peer,"interface")){peers.push((link.clone(),peer.clone()))}}}}}
  if !endpoint_found{return vec![self.finish(st,returning,"indeterminate","selected egress has no explicit topology link")]}
  if peers.len()!=1{return vec![self.finish(st,returning,"indeterminate","selected route does not identify exactly one linked peer as its next hop")]}
  let(link,peer)=peers.remove(0);let last=st.hops.len()-1;st.hops[last]["link"]=link_json(&link);let le=link_refs(&self.nodes,&link);if !le.is_empty(){st.hops[last]["evidence"].as_array_mut().unwrap().extend(le)}st.current=peer;self.walk(st,returning)
 }
 fn combine_returns(&mut self,forward:State,rows:Vec<Value>)->Value{
  let id=branch_id(&forward.hops,&forward.candidate_ids);let mut returns=Vec::new();let mut all_return=!rows.is_empty();let mut all_blocked=!rows.is_empty();
  for row in rows {let Some(mut ret)=row["returns"][0].as_object().cloned()else{all_return=false;all_blocked=false;continue};let mut rv=Value::Object(std::mem::take(&mut ret));let old_id=s(&rv,"id").to_owned();rv["id"]=json!(format!("{id}-return-{old_id}"));
   if s(&rv,"disposition")!="modeled_forward"||s(&rv,"session_status")!="established"{all_return=false}
   if s(&rv["direction"],"disposition")=="modeled_forward"&&!exact_reverse(&forward.hops,&rv["direction"]){rv["asymmetric"]=json!(true);rv["disposition"]=json!("indeterminate");rv["reasons"].as_array_mut().unwrap().push(json!("return node, link, interface, or tuple sequence does not exactly reverse the forward branch"));all_return=false}
   if s(&rv,"disposition")!="blocked"{all_blocked=false}returns.push(rv);
  }
  let disp=if all_return&&self.complete&&!self.truncated{"modeled_round_trip"}else if all_blocked&&self.complete{"blocked"}else{"indeterminate"};let reasons=if disp=="modeled_round_trip"{vec![]}else{vec!["return route or established session state is asymmetric, blocked, or unproven"]};
  json!({"id":id,"forward":{"disposition":"modeled_forward","hops":forward.hops,"final_flow":flow_tuple(&forward.flow),"reasons":forward.reasons},"returns":returns,"disposition":disp,"reasons":reasons})
 }
}
fn merge_session(current:&str,next:&str)->String{if current=="unproven"||next=="unproven"{"unproven".into()}else if current=="asymmetric_unknown"||next=="asymmetric_unknown"{"asymmetric_unknown".into()}else if !current.is_empty(){current.into()}else{next.into()}}
fn zero_flow()->Value{json!({"scope":{"snapshot_id":"","device_id":"","context":""},"source":"","destination":"","protocol":""})}
fn exact_reverse(forward:&[Value],reverse:&Value)->bool{
 let rh=a(reverse,"hops");if s(reverse,"disposition")!="modeled_forward"||forward.is_empty()||forward.len()!=rh.len(){return false}
 for (i,r) in rh.iter().enumerate(){let f=&forward[forward.len()-1-i];let fp=&f["path"];let rp=&r["path"];
  if s(r,"node_id")!=s(f,"node_id")||!s(rp,"egress_interface").eq_ignore_ascii_case(s(fp,"ingress_interface"))||flow_tuple(&rp["original_flow"])!=reverse_flow(&fp["egress_flow"],&rp["scope"])||flow_tuple(&rp["egress_flow"])!=reverse_flow(&fp["original_flow"],&rp["scope"]){return false}
  if i<rh.len()-1{let prior=&forward[forward.len()-2-i];if r["link"].is_null()||prior["link"].is_null(){return false}let same=(r["link"]["a"]==prior["link"]["a"]&&r["link"]["b"]==prior["link"]["b"])||(r["link"]["a"]==prior["link"]["b"]&&r["link"]["b"]==prior["link"]["a"]);if !same{return false}}else if !r["link"].is_null(){return false}
 }true
}

pub fn explore_network(input:Value)->Result<Value,String>{
 let mut out=json!({"engine_contract_version":"graphlayer-network-exploration-v1","original_flow":flow_tuple(&input["flow"]),"branches":[],"disposition":"indeterminate","complete":true,"truncated":false,"reasons":[],"assumptions":["configuration and topology are supplied evidence, not live device or link state","only explicitly linked IPv4 paths and bounded route choices are explored","equal-best route choices are configured alternatives; this does not prove ECMP is enabled or predict a hardware hash/session choice","return session state is modeled only when the exact reverse tuple and selected forward state are supported"]});
 let nodes=a(&input,"nodes");let links=a(&input,"links");if nodes.is_empty()||nodes.len()>16||links.len()>32{out["complete"]=json!(false);out["reasons"]=json!(["topology exceeds supported node or link limits"]);return Ok(out)}
 let src=&input["source"];let dst=&input["destination"];if s(src,"node_id").is_empty()||s(src,"interface").is_empty()||s(dst,"node_id").is_empty()||s(dst,"interface").is_empty()||s(src,"node_id").len()>64||s(dst,"node_id").len()>64||s(src,"interface").len()>48||s(dst,"interface").len()>48{return Err("source and destination node/interface are required".into())}
 if super::policy::validate_scope(json!({"scope":input["flow"]["scope"]})).is_err(){return Err("flow scope: snapshot_id, device_id, and context are required".into())}
 let mut by_id=HashMap::new();for n in nodes{let id=s(&n,"id");if id.is_empty()||id.len()>64||by_id.contains_key(id){out["complete"]=json!(false);out["reasons"]=json!(["topology contains a missing, overlong, or duplicate node identifier"]);return Ok(out)}if super::policy::validate_scope(json!({"scope":n["scope"]})).is_err(){return Err(format!("node {} scope: snapshot_id, device_id, and context are required",id))}by_id.insert(id.to_owned(),n);}
 if !by_id.contains_key(s(src,"node_id"))||!by_id.contains_key(s(dst,"node_id")){out["complete"]=json!(false);out["reasons"]=json!(["source or destination node is absent from topology"]);return Ok(out)}
 for l in &links{if s(&l["a"],"node_id").is_empty()||s(&l["b"],"node_id").is_empty()||s(&l["a"],"interface").is_empty()||s(&l["b"],"interface").is_empty()||s(&l["a"],"node_id").len()>64||s(&l["b"],"node_id").len()>64||s(&l["a"],"interface").len()>48||s(&l["b"],"interface").len()>48||s(&l["a"],"node_id")==s(&l["b"],"node_id")||!link_facts(&by_id,l){out["complete"]=json!(false);out["reasons"]=json!(["topology contains an invalid, missing, or unresolved link"]);return Ok(out)}}
 if scope(&input["flow"]["scope"])!=scope(&by_id[s(src,"node_id")]["scope"]){return Err("flow scope must match the source node scope".into())}
 if !connected(&by_id[s(src,"node_id")],s(src,"interface"),s(&input["flow"],"source")){out["complete"]=json!(false);out["reasons"]=json!(["source address is not proven on the selected ingress interface subnet"]);return Ok(out)}
 let state=State{current:src.clone(),flow:flow_tuple(&input["flow"]),hops:vec![],visited:HashSet::new(),forward_by_node:HashMap::new(),candidate_ids:vec![],session:String::new(),asymmetric:false,disposition:"indeterminate".into(),reasons:vec![]};let mut ctx=Ctx{input,nodes:by_id,links,evaluations:0,terminal:0,truncated:false,complete:true,reasons:vec![]};let branches=ctx.walk(state,false);out["branches"]=json!(branches);
 let mut rows=a(&out,"branches");rows.sort_by(|x,y|s(x,"id").cmp(s(y,"id")));out["branches"]=json!(rows);let all_blocked=!rows.is_empty()&&rows.iter().all(|x|s(x,"disposition")=="blocked");let all_round=!rows.is_empty()&&rows.iter().all(|x|s(x,"disposition")=="modeled_round_trip");out["disposition"]=json!(if all_blocked&&ctx.complete&&!ctx.truncated{"blocked"}else if all_round&&ctx.complete&&!ctx.truncated{"modeled_round_trip"}else{"indeterminate"});out["complete"]=json!(ctx.complete);out["truncated"]=json!(ctx.truncated);out["reasons"]=json!(ctx.reasons);Ok(out)
}
