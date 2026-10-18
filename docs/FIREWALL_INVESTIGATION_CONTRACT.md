# Local firewall investigation contract

This milestone delivers one actual Cisco ASA running-config → parsed ACL → evidence snapshot → policy decision → browser workflow. It is a local, single-user application. It is not an enterprise deployment or an end-to-end reachability simulator.

## Packaging decision

Use a Go executable with embedded HTML/CSS/JavaScript and same-origin JSON APIs. This keeps offline installation and the first real workflow reproducible without adding a second server or dependency stack. The thin UI can move to the planned TypeScript application later without changing the snapshot/evaluation boundary. Do not reuse the untracked older UI or its mock endpoints.

## Parser boundary

Package `firewall` exposes `ParseASAConfig([]byte) (*Config, error)`. It repairs and uses a native running-config parser in `vendors/cisco/asa`, rather than interpreting the old show-output parser as a config parser. Config contains vendor/parser version, hostname/context, original artifact SHA-256, sanitized source lines, evidence locators, attached ACL chains, warnings and supported capabilities. Only sanitized lines may be persisted or returned.

Each chain has stable ID, ACL name, interface/direction, completeness, ordered normalized `policy.Rule` values and evidence references. Physical line numbers and ACE order are distinct. Address/network objects and bounded nested network groups resolve into literal predicates; multiple members can expand one ACE into contiguous equivalent rules. Any unknown ACE, ambiguous attachment/context, cyclic/unresolved object or expansion limit must stay explicit and conservative. Never silently omit a potentially decisive ACE. Bindings are required for supported through-the-box ACL evaluation. No attempt is made to combine all ASA policy stages into a device-wide permit verdict.

## HTTP API (all same-origin, authenticated local browser session)

`GET /api/session`: initialize browser access; return `{csrf_token}` and set a protected HttpOnly SameSite session cookie. All API routes require that session. Mutating/pure evaluation POSTs additionally require a matching Origin and `X-CSRF-Token`.

`GET /api/snapshots`: `{snapshots:[SnapshotSummary]}`. Summary fields: `id`, `created_at`, `vendor`, `parser_version`, `hostname`, `context`, `artifact_sha256`, `sanitized_sha256`, `chain_count`, `rule_count`, `warning_count`.

`POST /api/imports`: JSON `{name:string, config:string}`. Return 201 `{snapshot:Snapshot}` or 200 for an identical existing snapshot. Request bounds and a single active import are enforced. Parse failure leaves earlier snapshots intact. No raw configuration is logged or persisted.

`GET /api/snapshots/{id}`: `{snapshot:Snapshot}`. A snapshot has the summary fields plus `name`, `chains`, `warnings`, `capabilities`, `source_lines`, `evidence`. Chain fields: `id`, `name`, `interface`, `direction`, `complete`, `rules`, `evidence_refs`, `warnings`. Source line fields: `number`, `text`. Evidence fields: `id`, `start_line`, `end_line`, `kind`, `summary`. Warning fields: `code`, `message`, `line` (optional). Rule shape is existing `policy.Rule` with normalized predicates and evidence IDs.

`POST /api/snapshots/{id}/evaluate`: `{chain_id,source,destination,protocol,source_port,destination_port}`. Server derives snapshot/device/context scope from the immutable snapshot and selected chain; clients cannot inject a different scope. Return `{investigation:Investigation}`. Investigation fields: `id`, `created_at`, `snapshot_id`, `chain_id`, `result` (existing `policy.Result`). Store the result for replay/history. Explicit chain selection means the result is an ACL policy decision, not a claim about routing/NAT/return traffic.

`GET /api/investigations`: `{investigations:[Investigation]}`. `GET /api/investigations/{id}` returns `{investigation:Investigation}`.

`GET /api/sample`: `{name,config}` for a labelled, sanitized, syntactically real ASA fixture. Loading the sample still passes it through the same import/parser/store path. It is not a prebuilt JSON result.

Errors: `{error:{code,message}}`; 400 validation, 403 session/origin/CSRF/Host, 404 missing resource, 413 bounded input, 422 config parse/unsupported import, 429 busy/quota, 500 storage/internal. Never include raw input, credential values, or underlying filesystem error text.

## Browser requirements

Import file or paste config; inspect parsed chains/capabilities/warnings; select an attached chain; enter a concrete flow; evaluate; show permit/deny/indeterminate, decisive/candidate rule, ordered considered rule trace and source evidence. Persist scope/snapshot identity in the view and show parser/source hashes and the policy-only qualification. Evidence is rendered as text, linked to 1-based physical line numbers. Import errors, partial data and service failures are distinct. Saved investigations can be reopened and exported as JSON. Provide responsive layout, keyboard labels/focus and readable tables. Never fabricate a result or silently default to a different snapshot/chain.

## Local trust and persistence

CLI listens on explicit numeric loopback IPs only. Reject foreign Host, foreign Origin, cross-site requests and invalid sessions. Use CSP, no external scripts/resources, request/time/concurrency bounds and redacted diagnostics. Persist sanitized, versioned, immutable snapshots and investigations atomically with restricted permissions. Validate loaded records and hashes; do not silently promote corrupt or old incompatible state. Raw upload credentials remain transient.

## Proof required

Go parser fixture and model tests, API/security/persistence tests, and a real browser flow: import actual vendor syntax → select attached ACL → allow HTTPS → deny SSH → unresolved object yields indeterminate → open exact source evidence → reload/reopen history. Include invalid/oversized input, wrong scope/chain, XSS/secret canaries, empty or malformed chains and failed import preserving the prior snapshot. CI must run the repaired ASA package and new app packages plus a pinned browser suite.
