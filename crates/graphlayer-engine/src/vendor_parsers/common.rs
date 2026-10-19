use serde_json::{json, Value};
use sha2::{Digest, Sha256};

pub fn sha256(data: &[u8]) -> String { format!("{:x}", Sha256::digest(data)) }
pub fn rule(id: &str, position: usize, enabled: bool, action: &str, src: &str, dst: &str, proto: &str, port: &str, refs: &[String]) -> Value {
    json!({"id":id,"position":position,"enabled":enabled,"action":action,"source":src,"destination":dst,"protocol":proto,"source_port":"","destination_port":port,"evidence_refs":refs})
}
pub fn warning(code: &str, message: &str, line: usize) -> Value {
    if line == 0 { json!({"code":code,"message":message}) } else { json!({"code":code,"message":message,"line":line}) }
}
pub fn evidence_id(hash: &str, line: usize) -> String { format!("sha256:{}", sha256(format!("{}:{}",hash,line).as_bytes())) }
pub fn source_lines(raw: &str, omission: &str) -> Vec<Value> { raw.replace("\r\n","\n").split('\n').enumerate().map(|(i,_)| json!({"number":i+1,"text":omission})).collect() }
pub fn add_evidence(evidence: &mut Vec<Value>, hash: &str, line: usize, kind: &str, summary: &str) -> String {
    let id=evidence_id(hash,line);
    if !evidence.iter().any(|x| x["id"]==id) { evidence.push(json!({"id":id,"start_line":line,"end_line":line,"kind":kind,"summary":summary})); }
    id
}
pub fn unique(values: impl IntoIterator<Item=String>) -> Vec<String> { let mut out=Vec::new(); for v in values { if !v.is_empty() && !out.contains(&v) { out.push(v); } } out }
pub fn ipv4_prefix(s: &str) -> Option<String> {
    use std::net::Ipv4Addr;
    if let Some((ip,bits))=s.split_once('/') { let a=ip.parse::<Ipv4Addr>().ok()?; let n=bits.parse::<u8>().ok()?; if n>32{return None} let mask=if n==0 {0} else {u32::MAX << (32-n)}; return Some(format!("{}/{}",Ipv4Addr::from(u32::from(a)&mask),n)); }
    if let Ok(a)=s.parse::<Ipv4Addr>() { return Some(format!("{}/32",a)); }
    if let Ok(a)=s.parse::<std::net::Ipv6Addr>() { return Some(format!("{}/128",a)); }
    None
}
pub fn mask_bits(s: &str) -> Option<u8> { let ip=s.parse::<std::net::Ipv4Addr>().ok()?; let x=u32::from(ip); let inv=!x; if inv & inv.wrapping_add(1)!=0 {None} else {Some(x.count_ones() as u8)} }
pub fn mask_prefix(ip: &str, mask: &str) -> Option<String> { let a=ip.parse::<std::net::Ipv4Addr>().ok()?; let bits=mask_bits(mask)?; let m=if bits==0{0}else{u32::MAX << (32-bits)}; Some(format!("{}/{}",std::net::Ipv4Addr::from(u32::from(a)&m),bits)) }
