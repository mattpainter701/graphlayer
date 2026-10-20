use graphlayer_engine::topology_layers::{layer_sample, project_layers};
use serde_json::json;
use std::time::{Duration, Instant};

// An end-to-end projection budget, not a throughput claim. CI emits the actual
// timings and serialized byte counts so changes can be compared on the same host.
#[test]
fn thousand_host_projection_budget() {
    let dataset = layer_sample(json!({})).expect("synthetic corpus")["dataset"].clone();
    let dataset_bytes = serde_json::to_vec(&dataset).unwrap().len();
    assert!(dataset_bytes < 4 << 20, "corpus exceeds HTTP input budget");
    let mut measurements = Vec::new();
    for view in ["l3", "zone", "l2"] {
        let mut timings = Vec::new();
        let mut response_bytes = 0;
        for _ in 0..5 {
            let started = Instant::now();
            let output = project_layers(json!({"dataset":dataset,"view":view,"limit":100}))
                .expect("validated Rust projection");
            timings.push(started.elapsed());
            assert_eq!(output["counts"]["hosts"], 1000);
            assert!(output["nodes"].as_array().unwrap().len() <= 140);
            assert!(output["hosts"].as_array().unwrap().len() <= 100);
            response_bytes = serde_json::to_vec(&output).unwrap().len();
            assert!(response_bytes < 4 << 20);
        }
        timings.sort();
        assert!(timings[4] < Duration::from_secs(5));
        eprintln!(
            "scale_projection view={view} hosts=1000 corpus_bytes={dataset_bytes} response_bytes={response_bytes} median_ms={:.2} max_ms={:.2}",
            timings[2].as_secs_f64() * 1000.0,
            timings[4].as_secs_f64() * 1000.0
        );
        measurements.push(json!({
            "view":view,"hosts":1000,"corpus_bytes":dataset_bytes,
            "response_bytes":response_bytes,
            "median_ms":timings[2].as_secs_f64()*1000.0,
            "max_ms":timings[4].as_secs_f64()*1000.0
        }));
    }
    if let Ok(directory) = std::env::var("GRAPHLAYER_SCALE_ARTIFACT_DIR") {
        let directory = std::path::PathBuf::from(directory);
        assert!(directory.is_absolute());
        std::fs::create_dir_all(&directory).unwrap();
        std::fs::write(
            directory.join("synthetic-topology-lab.json"),
            serde_json::to_vec_pretty(&dataset).unwrap(),
        )
        .unwrap();
        std::fs::write(
            directory.join("projection-measurements.json"),
            serde_json::to_vec_pretty(&measurements).unwrap(),
        )
        .unwrap();
    }
}
