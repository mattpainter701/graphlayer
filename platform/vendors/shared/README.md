# Shared Vendor Utilities

This directory contains cross-vendor utilities and helper functions used across multiple vendor implementations.

## Files

### evidence_hashing.go
**Purpose:** Evidence metadata and cryptographic hashing for data provenance

**Functions:**
- `HashEvidence(data []byte) string` - SHA-256 hash generation
- `CreateEvidenceMetadata(source, timestamp, hash string) EvidenceMetadata` - Evidence tracking
- `VerifyEvidence(data []byte, expectedHash string) bool` - Hash verification

**Use Cases:**
- MAC address table evidence tracking
- Configuration change detection
- Data integrity verification
- Audit trail generation

**Example:**
```go
import "github.com/mattpainter701/graphlayer/platform/vendors/shared"

// Hash collected data
macTableOutput := "..."
hash := shared.HashEvidence([]byte(macTableOutput))

// Create evidence metadata
evidence := shared.CreateEvidenceMetadata(
    "show mac address-table",
    time.Now().Format(time.RFC3339),
    hash,
)

// Store with topology data
macEntry.Evidence = evidence
```

### evidence_hashing_test.go
**Purpose:** Unit tests for evidence hashing functionality

**Coverage:**
- Hash generation consistency
- Evidence metadata creation
- Hash verification
- Edge cases (empty data, large data)

### suggestions_parsers.go
**Purpose:** Legacy parser suggestions and helper functions

**Note:** This file contains early parser prototypes and may be refactored in future releases.

## Package Structure

```go
package parsers  // Shared utilities use parsers package

import (
    "crypto/sha256"
    "encoding/hex"
    "time"
)
```

## Testing

Run shared utility tests:
```bash
cd vendors/shared
go test -v
```

## Usage Across Vendors

### Cisco
Used in:
- `vendors/cisco/ios/cisco_ios_mappers.go` - MAC table evidence
- `vendors/cisco/nxos/cisco_nxos_mappers.go` - FDB entry evidence

### Palo Alto
Used in:
- `vendors/paloalto/firewall/paloalto_mappers.go` - Security policy evidence

### Fortinet
Used in:
- `vendors/fortinet/fortigate/fortinet_mappers.go` - Configuration evidence

## Evidence Metadata Structure

```go
type EvidenceMetadata struct {
    Source      string    // Command or API endpoint
    Timestamp   time.Time // Collection timestamp
    Hash        string    // SHA-256 hash
    CollectorID string    // Device/collector identifier
    Confidence  string    // "high", "medium", "low"
}
```

## Best Practices

### When to Use Evidence Hashing

✅ **Use for:**
- MAC address table entries (L2 topology)
- ARP table entries (IP-MAC bindings)
- Configuration snapshots
- Security event data
- Audit-critical data

❌ **Don't use for:**
- Transient operational stats (CPU, memory)
- High-frequency metrics
- Non-critical logs
- Temporary cache data

### Hash Algorithm Choice

**SHA-256** is used because:
- FIPS 140-2 compliant
- Collision-resistant
- Fast enough for network data volumes
- Standard in security tools

### Performance Considerations

- Hashing overhead: ~1-2ms per 100KB
- Memory: ~32 bytes per hash
- Recommended: Hash at collection time, not on every read

## Integration with L2 Models

Evidence metadata integrates with L2 topology models:

```go
// FDBEntry with evidence
type FDBEntry struct {
    MAC         string
    VLAN        int
    Interface   string
    Evidence    EvidenceMetadata  // Provenance tracking
    Confidence  string            // Derived from evidence
}
```

## Future Enhancements

Planned additions:
1. **Chain of Custody**: Multi-hop evidence tracking
2. **Evidence Signing**: Digital signatures for audit compliance
3. **Evidence Aggregation**: Combine multiple evidence sources
4. **Evidence Expiry**: Time-based evidence validity
5. **Merkle Trees**: Efficient evidence verification for large datasets

## Compliance Support

Evidence hashing supports:
- **NERC CIP-010-4**: Configuration Change Management
- **PCI DSS 10.2**: Automated Audit Trails
- **NIST CM-2/CM-3**: Baseline Configuration & Change Control
- **SOC 2**: Data integrity and change tracking

## Contributing

When adding new shared utilities:
1. Ensure true cross-vendor applicability
2. Add comprehensive unit tests
3. Document performance characteristics
4. Update this README with new functions
5. Consider backward compatibility

## Contact

For questions about shared utilities:
- Check main project documentation
- Review L2 model specifications
- See integration guides for usage examples
