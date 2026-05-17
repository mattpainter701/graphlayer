# Topology Record Model v2

Sprint 1 introduces a normalized topology record layer that can sit above the existing vendor parsers without forcing a parser rewrite. The v2 schema is intentionally record-oriented: every topology fact is emitted as a typed record with the same metadata envelope for schema versioning, evidence provenance, and confidence scoring.

## Schema version

Current schema identifier: `topology.record.v2`

Go constant: `models.TopologySchemaVersionV2`

JSON Schema: `docs/schemas/topology-record-v2.schema.json`

The schema identifier is stored on every record at `meta.schema_version`; consumers should reject or route records whose version is not explicitly supported.

## Universal record kinds

The v2 model currently covers the Sprint 1 universal topology set:

- `device` -> `models.DeviceV2`
- `interface` -> `models.InterfaceV2`
- `link` -> `models.LinkV2`
- `vlan` -> `models.VLANV2`
- `route` -> `models.RouteV2`
- `security_policy` -> `models.SecurityPolicyV2`
- `nat_rule` -> `models.NATRuleV2`
- `vpn` -> `models.VPNV2`

Each type implements `models.TopologyRecordV2`, which exposes:

- `TopologyKind() TopologyRecordKindV2`
- `TopologyMetadata() *TopologyRecordMetaV2`

## Metadata envelope

Every v2 record has a `meta` field:

```json
{
  "schema_version": "topology.record.v2",
  "record_kind": "device",
  "record_id": "device:core1",
  "device_id": "device:core1",
  "virtual_context": "default",
  "source_vendor": "cisco",
  "source_platform": "ios-xe",
  "evidence": [],
  "confidence": {
    "overall": 0.9,
    "per_attribute": { "hostname": 0.9 },
    "tier": "high",
    "evidence_sources": ["config"],
    "evidence_count": 1,
    "corroboration_boost": 0,
    "staleness_decay": 0,
    "last_updated": "2026-05-17T00:00:00Z",
    "computed_by": "confidence-v2"
  },
  "created_at": "2026-05-17T00:00:00Z",
  "updated_at": "2026-05-17T00:00:00Z"
}
```

`meta.evidence` must always be present. If a parser cannot provide provenance yet, emit an empty array rather than omitting the field. `meta.confidence` must always be present; helpers can derive a baseline profile from evidence.

## Evidence provenance

`models.EvidenceRefV2` captures where a record or field came from:

- `source_type`: one of `config`, `operational`, `inferred`, `connector`, `manual`
- `source_system`: parser/connector/engine name such as `cisco-ios-parser` or `netbox`
- optional collector/parser/vendor/platform/device metadata
- optional command, file path, config source block, line range, and `sha256:` snippet hash
- `fields`: attribute names backed by that evidence reference
- `quality`: parser-local quality score from 0.0 to 1.0
- `observed_at`: when the evidence was observed or collected

Existing stanza hashing from `platform/vendors/shared` can populate `source_block` and `source_snippet_hash`.

## Confidence scoring primitives

The v2 records reuse the existing `models.ConfidenceProfile` and `models.ConfidenceSource` primitives. New helpers added for v2:

- `NewTopologyRecordMetaV2(kind, recordID, evidence, confidence)`
- `NormalizeEvidenceRefsV2(evidence)`
- `ConfidenceProfileFromEvidenceV2(evidence, fields)`
- `ValidateTopologyRecordMetaV2(meta, expectedKind)`

The default evidence-derived profile computes field scores from `SourceWeights[source_type] * evidence.quality`, clamps scores into `[0, 1]`, and rolls them up with the existing confidence tier logic.

## Parser migration approach

Do not refactor all vendor parsers in one pass. Sprint 1 adds low-risk adapters that map existing models into v2 records:

- `LegacyDeviceToV2`
- `LegacyInterfaceToV2`
- `LegacyL2VLANToV2`
- `LegacyRoutingEntryToV2`
- `LegacyFirewallPolicyToV2`
- `LegacyNATToV2`
- `LegacyTunnelToVPNV2`

Parser owners can keep existing parse output stable, then emit v2 records at integration boundaries or storage/export boundaries. When parser code already has stanza hashes, pass them as `EvidenceRefV2`; otherwise call adapters with `nil` evidence to get a valid v2 record with an empty evidence array and low/none confidence.

## Example

```go
evidence := []models.EvidenceRefV2{{
    SourceType:        models.ConfidenceSourceConfig,
    SourceSystem:      "cisco-ios-parser",
    Parser:            "ios-v1",
    Vendor:            "cisco",
    DeviceName:        "core1",
    SourceBlock:       "hostname core1",
    SourceSnippetHash: "sha256:...",
    Fields:            []string{"hostname", "vendor"},
    Quality:           1.0,
}}

device := models.LegacyDeviceToV2(models.Device{
    ID:           "device:core1",
    Hostname:     "core1",
    Vendor:       "cisco",
    Version:      "17.9",
    ManagementIP: "192.0.2.10",
}, evidence)
```

## Validation and tests

Run from `platform`:

```bash
go test ./models ./scoring
```

For full platform validation:

```bash
go test ./...
```
