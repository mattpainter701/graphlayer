# Enterprise foundation: current implementation

This is the first implementation increment from [the overhaul plan](ENTERPRISE_OVERHAUL_PLAN.md), not the completed enterprise application. It adds an offline policy evaluation contract and repairs demonstrated confidence/reference-data/build validation defects. The detailed [backlog](ENTERPRISE_OVERHAUL_BACKLOG.md) remains the delivery map.

## Replay the policy proof

Requires the Go version declared in `platform/go.mod` or a compatible toolchain. From `platform/`:

```powershell
go run ./cmd/graphlayer-policy --bundle ./policy/fixtures/allow-https.bundle.json --flow ./policy/fixtures/allow-https.flow.json
```

The synthetic sample returns a policy-only `permit` attributed to `allow-web`. Its evidence identifiers demonstrate linking; they are not verified configuration hashes or evidence of a real device. The result carries its contract version, snapshot/device/context, input tuple, considered rules, decisive/candidate rule, evidence references, supported semantics, unsupported conditions and assumptions.

Supported inputs are already-normalized single-device policy bundles. The implemented subset covers literal IPv4/IPv6 addresses and prefixes, address `any`, concrete numeric IP protocols or the documented aliases, TCP/UDP source/destination ports and ranges, enabled state, explicit rule order and complete-chain defaults. Scope mismatch and malformed tuples are errors. Earlier unresolved candidate rules and incomplete bundles prevent a definitive verdict. Known independent nonmatches can safely eliminate unresolved candidates.

There is no vendor configuration adapter, evidence storage, named-object expansion, route/NAT/return-path simulation, authentication service, operator UI or device provisioning in this increment. Evidence references and completeness are caller assertions. A permitted policy result cannot establish that the network will forward the packet. Full capabilities and input details are in [the package guide](../platform/policy/README.md).

CLI inputs are bounded to 8 MiB each and policy rule count is bounded. JSON field/trailing-data checks reject malformed bundles. Validation failures exit nonzero; a valid indeterminate decision is a successful evaluation with an unresolved result. This command opens only the supplied local files and makes no network/device changes.

## Confidence behavior changes

- Derived confidence is bounded by the decayed parent rather than becoming stronger merely through propagation.
- Staleness scales scores once, preserves its diagnostic fraction and keeps detailed attribute scores consistent. Recomputing the same target age does not compound the decay.
- Corroboration requires explicit agreement on the same field value. Matching score-map keys cannot establish agreement; score-only `CorroborateAcrossSources` callers receive no automatic boost and must supply value-bearing evidence through `ApplyCorroboration`.
- Reusing the same corroboration evidence does not accumulate bonuses. The current evidence set replaces the prior contribution; removing agreement removes the old contribution.
- Filtering for corroboration uses an explicit source count, not the count of scored attributes.

Consumers of the old JSON confidence model should account for the added corroborating-source count and the corrected interpretation of decay. Previously persisted derived confidence needs recomputation from its original evidence; changing the model alone cannot repair old stored scores.

## Reference catalogs and reproducible checkout

Small pytest and embedded reference text inputs are ordinary Git text instead of missing LFS objects. The IANA/IEEE directory intentionally contains bounded common defaults and schema placeholders, not a full synchronized registry. Unknown protocol/EtherType lookups remain unknown. See [reference-data limitations](../platform/reference/iana/data/README.md). Full registry acquisition/versioning remains work in the plan.

CI reads the Go module version, installs declared Python test dependencies, runs valid scoped vet/fuzz commands and records benchmark output. The unsupported comparator `--bench-compare` invocation is removed; benchmark artifacts are informational until a calibrated regression baseline is implemented. Docker QA remains a comparator CLI self-test. It does not validate a running application server. Normal compose/connector-service packaging remains a separate repair item.

## Verification scope

From `platform/`, the supported foundation packages are:

```powershell
go test ./correlation/... ./comparison/... ./graph/... ./intelligence/... ./scoring/... ./vendors/shared/... ./models/... ./reference/iana/... ./policy/... ./cmd/graphlayer-policy/...
go vet ./correlation/... ./comparison/... ./graph/... ./intelligence/... ./scoring/... ./vendors/shared/... ./models/... ./reference/iana/... ./policy/... ./cmd/graphlayer-policy/...
```

From the repository root:

```powershell
python -m pip install -r requirements-dev.txt
python -m pytest tests/ -q
```

The coverage job retains a 70% total threshold on its explicit supported foundation package set. The new evaluator and CLI are included. This is a scoped green gate; it is not a claim that `go test ./...` or every nested module passes.

Measured discovery found pre-existing vendor package build failures and missing nested orchestration module dependencies. Those failures are recorded in the [baseline audit](ENTERPRISE_OVERHAUL_AUDIT.md) and block real vendor integration. The legacy vendor-aware decision engine is not wired to the new evaluator. Recovering parser/model contracts, then proving a real config-to-investigation workflow, is the next milestone.

Local verification uses an isolated Go toolchain and Python virtual environment outside the repository. Docker Desktop's Linux daemon was unavailable on the development host; the GitHub QA job supplies the exact container self-test evidence. No production deployment or external device collection is performed by this change.

Final local validation for this increment: supported Go tests passed uncached at 74.5% merged statement coverage; supported `go vet` passed; Python reported 73 passed (including benchmark cases, with existing fixture warnings); actual CLI replays returned permit for HTTPS, deny for SSH, and indeterminate for an incomplete bundle. Independent review found and verified the repair of repeated corroboration inflation. The comparator CLI self-test also ran successfully. These results apply to the declared scope and synthetic inputs, not the incomplete legacy vendor integration or a competitive benchmark.
