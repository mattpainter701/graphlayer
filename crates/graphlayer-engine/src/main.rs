#![forbid(unsafe_code)]

use graphlayer_engine::{parse_asa_path, Input};
use serde::{Deserialize, Serialize};
use std::io::{self, Read, Write};
use std::panic;

const PROTOCOL: &str = "graphlayer-engine-v1";
const OPERATION: &str = "parse_asa_path";
const MAX_INPUT: usize = 12 * 1024 * 1024;
const MAX_OUTPUT: usize = 8 * 1024 * 1024;
const MAX_SOURCE_BYTES: usize = 2 * 1024 * 1024;
const MAX_LINES: usize = 50_000;
const MAX_LINE_BYTES: usize = 8192;

#[derive(Deserialize)]
#[serde(deny_unknown_fields)]
struct Request {
    protocol_version: String,
    operation: String,
    input: Input,
}
#[derive(Serialize)]
struct Response<'a, T> {
    protocol_version: &'a str,
    operation: &'a str,
    result: T,
}
#[derive(Serialize)]
struct ErrorResponse<'a> {
    protocol_version: &'a str,
    operation: &'a str,
    error: &'a str,
}

fn main() {
    // Panics are converted to a fixed protocol error and never leak details to stderr/stdout.
    panic::set_hook(Box::new(|_| {}));
    let outcome = panic::catch_unwind(run);
    if outcome.is_err() {
        emit_error("internal_error");
    }
}

fn run() {
    let mut input = Vec::new();
    let read_result = io::stdin()
        .take((MAX_INPUT + 1) as u64)
        .read_to_end(&mut input);
    if read_result.is_err() {
        emit_error("invalid_request");
        return;
    }
    if input.len() > MAX_INPUT {
        emit_error("input_too_large");
        return;
    }
    let request: Request = match serde_json::from_slice(&input) {
        Ok(value) => value,
        Err(_) => {
            emit_error("invalid_request");
            return;
        }
    };
    if request.protocol_version != PROTOCOL || request.operation != OPERATION {
        emit_error("unsupported_request");
        return;
    }
    if !valid_input(&request.input) {
        emit_error("invalid_request");
        return;
    }
    let result = parse_asa_path(request.input);
    let response = Response {
        protocol_version: PROTOCOL,
        operation: OPERATION,
        result,
    };
    let bytes = match serde_json::to_vec(&response) {
        Ok(bytes) => bytes,
        Err(_) => {
            emit_error("internal_error");
            return;
        }
    };
    if bytes.len() + 1 > MAX_OUTPUT {
        emit_error("output_too_large");
        return;
    }
    let mut stdout = io::stdout().lock();
    let _ = stdout.write_all(&bytes);
    let _ = stdout.write_all(b"\n");
}

fn valid_input(input: &Input) -> bool {
    if input.artifact_sha256.len() != 64
        || !input
            .artifact_sha256
            .bytes()
            .all(|b| b.is_ascii_digit() || (b'a'..=b'f').contains(&b))
    {
        return false;
    }
    if input.lines.len() > MAX_LINES {
        return false;
    }
    let mut source_bytes = 0usize;
    for (index, line) in input.lines.iter().enumerate() {
        if line.number != (index as i32) + 1 {
            return false;
        }
        if line.text.len() > MAX_LINE_BYTES {
            return false;
        }
        let separator = usize::from(index + 1 < input.lines.len());
        source_bytes = match source_bytes.checked_add(line.text.len() + separator) {
            Some(total) if total <= MAX_SOURCE_BYTES => total,
            _ => return false,
        };
    }
    true
}

fn emit_error(code: &'static str) {
    let response = ErrorResponse {
        protocol_version: PROTOCOL,
        operation: OPERATION,
        error: code,
    };
    if let Ok(bytes) = serde_json::to_vec(&response) {
        let mut stdout = io::stdout().lock();
        let _ = stdout.write_all(&bytes);
        let _ = stdout.write_all(b"\n");
    }
}
