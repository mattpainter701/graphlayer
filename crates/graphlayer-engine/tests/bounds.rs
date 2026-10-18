use graphlayer_engine::{parse_asa_path, ConfigLine, Input, MAX_PATH_INTERFACES, MAX_PATH_ROUTES};
use std::collections::BTreeMap;

fn input(lines: Vec<String>) -> Input {
    Input {
        artifact_sha256: "0".repeat(64),
        lines: lines
            .into_iter()
            .enumerate()
            .map(|(index, text)| ConfigLine {
                number: index as i32 + 1,
                text,
            })
            .collect(),
        network_objects: BTreeMap::new(),
    }
}

#[test]
fn oversized_interface_inventory_is_bounded_and_incomplete() {
    let mut lines = Vec::new();
    for index in 0..=MAX_PATH_INTERFACES {
        lines.push(format!("interface Gi0/{index}"));
        lines.push(format!(" nameif interface-{index}"));
        lines.push(" ip address 10.0.0.1 255.255.255.0".into());
    }
    let output = parse_asa_path(input(lines));
    assert_eq!(output.path.interfaces.len(), MAX_PATH_INTERFACES);
    assert!(!output.path.routing_complete);
    assert!(output
        .path
        .warnings
        .iter()
        .any(|warning| warning.code == "path_interface_limit"));
}

#[test]
fn oversized_route_inventory_is_bounded_and_incomplete() {
    let mut lines = vec![
        "interface Gi0/0".into(),
        " nameif outside".into(),
        " ip address 192.0.2.1 255.255.255.0".into(),
    ];
    for index in 0..=MAX_PATH_ROUTES {
        lines.push(format!(
            "route outside 10.{}.{}.0 255.255.255.0 192.0.2.254",
            index / 256,
            index % 256
        ));
    }
    let output = parse_asa_path(input(lines));
    assert_eq!(output.path.routes.len(), MAX_PATH_ROUTES);
    assert!(!output.path.routing_complete);
    assert!(output
        .path
        .warnings
        .iter()
        .any(|warning| warning.code == "path_route_limit"));
}
