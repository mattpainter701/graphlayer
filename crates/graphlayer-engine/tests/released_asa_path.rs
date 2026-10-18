use graphlayer_engine::{parse_asa_path, Input};
use serde::Deserialize;
use serde_json::Value;

const RELEASED_GO_CORPUS: &str = include_str!("../testdata/released-asa-path.json");

#[derive(Deserialize)]
struct Fixture {
    input: Input,
    output: Value,
}

#[test]
fn matches_released_go_asa_path_outputs() {
    let fixtures: Vec<Fixture> = serde_json::from_str(RELEASED_GO_CORPUS)
        .expect("released Go ASA path corpus should be valid JSON");
    assert!(
        fixtures.len() >= 72,
        "expected at least 72 captured Go cases, found {}",
        fixtures.len()
    );

    for (index, fixture) in fixtures.into_iter().enumerate() {
        let actual = serde_json::to_value(parse_asa_path(fixture.input))
            .expect("Rust ASA path output should serialize to JSON");
        assert_eq!(
            actual, fixture.output,
            "Rust output differs from released Go case {index}"
        );
    }
}
