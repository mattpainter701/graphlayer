# Checking a private configuration corpus

`scripts/check-private-corpus.py` inventories a local directory and, when given
the Rust engine, parses supported text configurations and checks parser output
structure. Keep the corpus and any report outside the source checkout. The
checker never prints source text, device names, evidence identifiers, or worker
responses. Its console output contains aggregate counts only. An optional JSON
report contains aggregate counts and SHA-256 hashes of input bytes, without
filenames or relative paths.

Build the Rust worker through the normal project workflow, then run the checker
with local paths:

```text
python scripts/check-private-corpus.py --corpus /path/to/private/configs --engine /path/to/graphlayer-engine --report /path/outside/checkout/inventory.json
```

The corpus can also be supplied through `GRAPHLAYER_PRIVATE_CORPUS`, and the
engine through `GRAPHLAYER_ENGINE`. If the engine is missing, parsing fails
closed; the checker does not build it or fall back to a Go or Python parser.

The inventory includes regular files and directories without following
symlinks. Candidate files have `.txt`, `.conf`, `.cfg`, `.xml`, or `.cli`
extensions, or no extension. Archives, other extensions, binary content, and
files larger than 2 MiB are counted as skips. The worker is called using its
JSON stdin protocol with a 30 second timeout and an 8 MiB response limit. For
each parsed result, the checker verifies the exact input-byte SHA-256, source
line and evidence ranges, and evidence references. It then asks Rust to run
`validate_inventory` and `validate_path_config` when parser metadata is
available.

To inspect file and extension counts without reading configuration contents or
requiring the Rust engine, use:

```text
python scripts/check-private-corpus.py --corpus /path/to/private/configs --inventory-only
```

Do not place private configurations, per-file paths, raw parser results, or
unredacted diagnostic output in tracked files or issue reports. The script's
unit tests use synthetic temporary files and do not launch the Rust worker.
