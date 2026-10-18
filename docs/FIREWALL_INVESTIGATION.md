# Firewall investigation: ASA configuration to evidence

This increment turns actual ASA running-config syntax into a saved, inspectable ACL policy decision. The Go executable embeds its browser UI and runs offline on loopback. It replaces the old prototype's disconnected upload and canned analysis workflow for this use case. It does not establish that a packet traverses a firewall or a network.

The subsequent [policy workspace increment](POLICY_WORKSPACE.md) adds service objects/groups, inventory search, dependency inspection and predicate explanations, with read compatibility for this initial release's stored records. That guide records current parser/result/storage versions and upgrade behavior.

The [routing/NAT increment](ROUTING_NAT_INVESTIGATIONS.md) adds a separate explicit-ingress path workflow and records its original parser/storage versions. Existing ACL-only investigations retain their original scope.

The [multi-firewall increment](MULTI_FIREWALL_INVESTIGATIONS.md) adds explicit topology, routing/NAT composition and configured return traces under an established-session assumption. It records the current workspace-v4 format and upgrade behavior.

## Run it

Use the Go version in `platform/go.mod`. From the repository root:

```powershell
cd platform
go run ./cmd/graphlayer-investigate --listen 127.0.0.1:8088
```

For a reusable executable:

```powershell
go build -o graphlayer-investigate.exe ./cmd/graphlayer-investigate
./graphlayer-investigate.exe --listen 127.0.0.1:8088 --data-dir C:/GraphLayer/my-investigations
```

On Linux, omit `.exe` and use a private dedicated data directory. The default data directory is `GraphLayer/investigations` below `os.UserConfigDir()` (normally `%AppData%` on Windows and `$XDG_CONFIG_HOME` or `~/.config` on Linux). An explicit directory must be empty or an existing GraphLayer workspace. The app refuses unrelated files, symlinks and concurrent writers. Close the process before reopening that workspace. Wildcard addresses, hostnames and non-loopback bindings are rejected; IPv6 `[::1]:8088` is supported. Open the exact URL printed by the process.

In the browser, load the labeled ASA sample or choose/paste an exported config, then import it. Inspect warnings and choose its attached ACL before evaluating. For the included sample, source `192.0.2.42`, destination `10.20.4.15`, TCP source port `52000` produces:

| Destination port | Result | Source-backed explanation |
|---|---|---|
| 443 | permit | First permit references the resolved client group and web-server object |
| 22 | deny | Explicit SSH deny matches |
| 8443 | indeterminate | A potentially matching ACE references an unresolved destination object |

Open a rule's evidence to inspect its physical source lines. Reload the page and reopen history to recover the saved snapshot, chain and evaluated tuple. Export writes the investigation result JSON; it does not export original credentials or prove device origin. The sample is sanitized, syntactically representative ASA text with synthetic canaries, not a customer capture or proof of device behavior.

## Supported semantics and boundaries

| Area | Implemented behavior |
|---|---|
| Input | ASA text running configuration, bounded and parsed independently from legacy `show access-list` output |
| Scope | Explicit attached interface ACL, hostname/context, artifact and parser identity; selected ACL policy only |
| Rule matching | Extended permit/deny, first-match order, inactive ACEs, IPv4/IPv6 literal/prefix and any/any4/any6 predicates |
| Protocol and ports | Concrete TCP/UDP/ICMP/ICMPv6/numeric protocols, rule-only IP wildcard, numeric `eq`/inclusive `range`, a bounded common named-port list |
| Address objects | Host/subnet objects, network objects, nested network groups; cycles, missing members, unsupported members and expansion limits remain unresolved |
| Unknowns | Potentially decisive unsupported predicates block later verdicts; omitted/ambiguous chain semantics cannot produce definitive verdicts |
| Defaults | Implicit deny only for a complete modeled supported interface chain; source evidence points to the attachment, not an invented deny command |
| Evidence | Original upload SHA-256, sanitized-source checksum, parser version, stable rule/evidence IDs and physical line locators |
| Storage | Sanitized immutable records, atomic publication, deduplication, bounded history and validated replay after restart |

“Parsed chain” means the parser captured the supplied chain under its supported contract. It cannot prove that an export contains every rule on the device, that the device is running that config, or that the source is authentic. Hashes identify bytes and detect accidental corruption; they are not signatures. Review collection completeness separately.

Enter the real addresses and ports used by ASA's ACL stage. Cisco documents extended ACL ordering, attachment and untranslated-address semantics in its [ASA access-rule guide](https://www.cisco.com/c/en/us/td/docs/security/asa/asa920/configuration/firewall/asa-920-firewall-config/access-acls.html). No address/port translation is inferred here. Physical ACE order in running-config is supported; explicit `line` insertion directives are retained conservatively rather than treated as sortable physical positions. Outbound, global/control-plane, ambiguous multiple contexts and other unsupported attachment semantics are reported conservatively. Service objects/groups, address ranges/FQDN/interface members, schedules, application/user identity and other unimplemented ACE modifiers cannot silently become wildcards or disappear.

Routes, NAT, stateful connection tracking, return traffic, VPNs, security-level defaults and multi-hop topology are not evaluated. ASA routing parsers were repaired against the canonical model so that the package compiles and typed process/route data is preserved; the investigation app does not use those routes to claim reachability. Other vendor packages and the nested orchestration modules are not certified by this milestone.

## Evidence and local trust

The parser holds raw upload bytes transiently to compute their hash and extract policy. Persisted and API-returned source keeps evidence-relevant policy lines; other lines are omitted, and sensitive commands are redacted. Source line numbers still refer to the original upload, so an evidence span may include omitted lines between dependencies. Object/ACL names and addresses remain sensitive network information. Use the workspace's restricted access accordingly.

The app sets Unix directory/file modes and Windows protected DACLs, takes an OS process lock, writes same-directory temporary files before rename, and validates version/checksum/identity/scope on load. Every loaded investigation is recomputed against its snapshot and compared with the saved result. Corrupt or incompatible workspaces fail explicitly. The current release can read the initial v1 format without rewriting it; see the policy workspace guide for explicit version compatibility. There is no general schema migration or certified power-loss recovery. Preserve the directory before moving it between app versions; application-level export currently covers results only. There is no UI deletion/retention management yet.

This is one local user's process with a random per-process browser session, HttpOnly SameSite cookie and CSRF token. Host, remote address, Origin and fetch-site checks, strict JSON, CSP, text-only dynamic rendering and bounded HTTP timeouts protect the local browser boundary. The service uses local HTTP. It has no enterprise user accounts, roles, tenant isolation, TLS ingress or remote access mode. OS administrators and the owning user remain trusted; this is not a protection boundary against a compromised local account. No device collection, device writes, external scripts or cloud services are required.

Current limits: 2 MiB config, 50,000 physical lines, 8,192 bytes per line, object nesting depth 16, 256 expansions per ACE, 10,000 normalized rules per chain, 32 snapshots and 256 unique saved investigations. HTTP JSON bodies are capped at 4 MiB and persisted records at 8 MiB. One import runs at a time. Limit/parse/storage failures leave the prior snapshot available. Identical imports and repeated identical investigations reuse their immutable record.

## Architecture decision and backlog impact

The [frozen HTTP/data contract](FIREWALL_INVESTIGATION_CONTRACT.md) separates the parser, normalized evaluator, evidence store and UI. A single embedded Go executable makes the first offline workflow deployable and testable without recovering the dirty older prototype or introducing an additional server. The browser code is deliberately thin. The planned TypeScript frontend can consume the same boundary later.

This is the first local M1 delivery slice, not completion of every enterprise M1 acceptance item:

| Backlog work | Evidence delivered | Remaining work |
|---|---|---|
| I01 | ASA running-config parser/adapter with bounded object expansion and conservative unknowns | Wider vendor/version corpus and source collection completeness |
| I02–I03 | Sanitized hashed snapshots, physical line locators, immutable atomic publication and failed-import preservation | Separately authorized encrypted raw evidence and collection provenance |
| I04–I06 | Synchronous bounded import, local persistence/locking and local session/scope checks | Durable jobs, migration/restore drill, enterprise identity/roles |
| I07–I09 | Validated same-origin API, error contract and responsive import/flow UI | Generated OpenAPI client, enterprise shell/search and asynchronous job UX |
| I10–I12 | Real policy investigation, evidence trace, saved replay, Go/API/browser tests | Expansion tree, broad device-certified golden corpus and enterprise outage/scale gates |

The next coherent increment is broader ASA semantics/corpus plus service/object inspection, followed by a second adapter under the same truthfulness contract. Durable import jobs and enterprise identity remain required before offering this as a shared service. Multi-hop routing/NAT work remains M3. Comparative superiority claims still require the independently measured M6 protocol.

## Reproduce the checks

From `platform/`:

```powershell
go test ./vendors/cisco/asa ./firewall ./policy ./investigation ./cmd/graphlayer-investigate
go vet ./vendors/cisco/asa ./firewall ./policy ./investigation ./cmd/graphlayer-investigate
```

From the repository root, with Go and Node available:

```powershell
npm ci
npx playwright install chromium
npm run test:e2e
```

Playwright builds the actual executable, starts it on loopback with an isolated disposable workspace, imports ASA text and exercises the live browser/API/store. It checks permit/deny/unknown, exact source evidence, reload/history/export, malformed import preservation, secret canaries, inert hostile source, request protections and responsive layout. It does not mock the evaluator. The harness refuses to clear paths outside its dedicated runtime folders and refuses to reuse an existing server.

CI runs the expanded scoped Go tests/vet with the unchanged 70% aggregate coverage gate, a Windows-native storage/parser check and a Linux Chromium browser job with screenshot/trace artifacts. Existing Python, comparator Docker QA, fuzz and benchmark jobs remain. The supported package manifest is explicit in `.github/workflows/test.yml`; `go test ./...` still includes unrelated legacy failures. Coverage is a regression gate, not evidence of device-semantic certification.
