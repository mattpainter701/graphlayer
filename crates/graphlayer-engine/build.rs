use std::env;
use std::fs;
use std::path::PathBuf;

const RESOURCE_ENV: &str = "GRAPHLAYER_VERSION_RESOURCE";
const RESOURCE_NAME: &str = "graphlayer-engine.res";

fn main() {
    println!("cargo:rerun-if-env-changed={RESOURCE_ENV}");

    let Some(resource) = env::var_os(RESOURCE_ENV) else {
        return;
    };
    let resource = PathBuf::from(resource);
    if !resource.is_absolute() {
        panic!("{RESOURCE_ENV} must be an absolute path to {RESOURCE_NAME}");
    }
    if resource.file_name().and_then(|name| name.to_str()) != Some(RESOURCE_NAME) {
        panic!("{RESOURCE_ENV} must name {RESOURCE_NAME}");
    }

    let target_os = env::var("CARGO_CFG_TARGET_OS").unwrap_or_default();
    let target_env = env::var("CARGO_CFG_TARGET_ENV").unwrap_or_default();
    if target_os != "windows" || target_env != "msvc" {
        panic!("{RESOURCE_ENV} is supported only for Windows MSVC builds");
    }

    let file_type = match fs::symlink_metadata(&resource) {
        Ok(metadata) => metadata.file_type(),
        Err(_) => panic!("{RESOURCE_ENV} must name an existing regular resource file"),
    };
    if !file_type.is_file() {
        panic!("{RESOURCE_ENV} must name an existing regular resource file");
    }

    let resource = match fs::canonicalize(&resource) {
        Ok(resource) => resource,
        Err(_) => panic!("{RESOURCE_ENV} could not be resolved"),
    };
    println!("cargo:rerun-if-changed={}", resource.display());
    // Microsoft LINK accepts .res files directly and invokes CVTRES to convert
    // them during linking. Restrict this argument to the worker binary so unit
    // tests, libraries, examples, and other package targets remain unaffected.
    println!(
        "cargo:rustc-link-arg-bin=graphlayer-engine={}",
        resource.display()
    );
}
