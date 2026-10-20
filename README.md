# GraphLayer

GraphLayer is a multi-vendor network topology graph engine. It ingests network
device configurations and operational state, normalizes topology data, builds
graph representations, and exports them for visualization and comparison.

The enterprise overhaul is defined in the [product and architecture plan](docs/ENTERPRISE_OVERHAUL_PLAN.md),
[execution backlog](docs/ENTERPRISE_OVERHAUL_BACKLOG.md), and [baseline audit](docs/ENTERPRISE_OVERHAUL_AUDIT.md).
The application now imports Cisco ASA running configurations into immutable, sanitized
evidence snapshots and evaluates concrete flows through a selected attached ACL. Its local
browser UI shows ordered predicate traces, physical source lines, warnings and saved investigations.
The [policy workspace](docs/POLICY_WORKSPACE.md) adds service objects/groups, searchable
rule inventories and typed object dependency inspection, while preserving older saved evidence.
The [routing/NAT workflow](docs/ROUTING_NAT_INVESTIGATIONS.md) adds explicit-ingress,
configuration-derived ASA paths with translated tuples, route selection and source-backed stages.
The [multi-firewall workflow](docs/MULTI_FIREWALL_INVESTIGATIONS.md) composes explicitly linked ASA snapshots,
checks configured forward and return paths, and explains the established-session assumption.
The [multi-vendor milestone](docs/MULTI_VENDOR_INVESTIGATIONS.md) adds native PAN-OS set-format and FortiOS CLI imports with their own policy/NAT semantics and mixed-vendor evidence traces.
The [path exploration milestone](docs/PATH_EXPLORATION.md) keeps branching routes and asymmetric configured returns separate, with a topology view, source evidence and immutable exploration history.
The [workspace and import foundation](docs/WORKSPACE_IMPORTS.md) adds sites, named snapshot sets, reviewed batch imports and a redesigned onboarding shell with atomic baseline publication.
The [topology workspace](docs/TOPOLOGY_WORKSPACE.md) separates workspace, imports,
investigation and topology pages, with device search, an interactive canvas and
snapshot-qualified configuration evidence.
See the [investigation guide](docs/FIREWALL_INVESTIGATION.md) for supported syntax, limits,
launch commands and tests. This is a single-user investigation application;
enterprise service deployment and end-to-end network reachability remain planned work.

Rust is required for new parser, connector and analysis-engine behavior. The
[language policy](docs/RUST_ARCHITECTURE.md) is enforced in CI. The local app's
ASA/PAN-OS/FortiOS parsing, model validation, policy, routing/NAT, return paths
and multi-firewall exploration execute in Rust. Connector acquisition and
normalization execute in Rust behind compatibility Go/Python APIs. Older
experimental vendor and graph libraries remain frozen outside this runtime.
See the
[Rust worker and packaging guide](docs/RUST_WORKER.md) for builds and tests.

## Code signing policy

Windows CI downloads are explicitly unsigned development builds. SignPath
Foundation is the selected route for preparing OSS release signing, subject to
eligibility and owner approval; no signing service or certificate is provisioned
yet. See the [Windows signing policy](docs/WINDOWS_SIGNING.md) and
[SignPath onboarding draft](docs/SIGNPATH_ONBOARDING.md) for the current status,
release gates and remaining license, eligibility and role decisions.

Private configuration collections can be inventoried and checked with the
[local corpus runner](docs/PRIVATE_CORPUS.md). Keep customer files outside the
checkout; public CI uses synthetic and sanitized released fixtures.

```powershell
cargo build --locked --release -p graphlayer-engine
$env:GRAPHLAYER_ENGINE = (Resolve-Path target/release/graphlayer-engine.exe).Path
cd platform
go run ./cmd/graphlayer-investigate --listen 127.0.0.1:8088
```

Open `http://127.0.0.1:8088`, choose **Sources & imports**, load the labeled ASA sample, import it, select its
`OUTSIDE-IN` chain, and evaluate a flow. The sample passes through the actual parser;
the application does not use precomputed demonstration results.

## 📁 Repository Structure

```
demo-data/
├── platform/                    # 🚀 MAIN PLATFORM CODE
│   ├── orchestration/           # Core orchestration engine
│   ├── vendors/                 # Vendor implementations (Cisco, Palo Alto, Fortinet)
│   ├── models/                  # Vendor-agnostic topology models
│   ├── integrations/            # Third-party integrations
│   └── docs/                    # Platform documentation
│
├── Cisco/                       # Cisco sample configurations
├── PaloAlto/                    # Palo Alto sample configurations
├── Fortinet/                    # Fortinet sample configurations
├── Juniper/                     # Juniper sample configurations
├── RunZero/                     # RunZero asset data
├── Claroty/                     # Claroty asset data
├── ForeScout/                   # ForeScout asset data
├── Nozomi/                      # Nozomi asset data
├── Tenable/                     # Tenable vulnerability data
└── sample_data/                 # Additional test data
```

## 🚀 Multi-Vendor Network Topology Platform

The main platform code is located in the **[`platform/`](platform/)** directory.

### Quick Links

- **📖 [Platform Documentation](platform/docs/getting-started/README.md)** - Complete framework documentation
- **🏗️ [Platform Architecture](platform/docs/FINAL_STRUCTURE.md)** - Directory structure and organization
- **🔧 [Integration Guide](platform/docs/getting-started/INTEGRATION_GUIDE.md)** - How to use the platform
- **🔌 [Connector Authoring Guide](platform/docs/getting-started/CONNECTOR_AUTHORING_GUIDE.md)** - Connector contract, normalization rules, and registry usage
- **📚 [Use Cases](platform/docs/getting-started/USE_CASES.md)** - Practical examples

### Vendor Support

The repository also contains older experimental vendor libraries. This table
describes their locations, not additional supported investigation imports:

| Vendor | Platforms | Location |
|--------|-----------|----------|
| **Cisco** | IOS, NX-OS, ASA | [`platform/vendors/cisco/`](platform/vendors/cisco/) |
| **Palo Alto** | PAN-OS, Panorama | [`platform/vendors/paloalto/`](platform/vendors/paloalto/) |
| **Fortinet** | FortiGate, FortiManager | [`platform/vendors/fortinet/`](platform/vendors/fortinet/) |

### Key Features

- ✅ **Multi-vendor topology discovery** (Cisco, Palo Alto, Fortinet)
- ✅ **L2/L3+ network visibility** (MAC tables, ARP, CDP/LLDP, routing)
- ✅ **Security policy mapping** (firewall rules, NAT, VPN)
- ✅ **Configuration backup & change detection**
- ✅ **Vendor-agnostic data models** (unified topology representation)
- ✅ **Centralized management integration** (Panorama, FortiManager)

### Quick Start

```bash
# Navigate to platform
cd platform

# Run the currently stable Go package checks
go test ./graph ./integrations/solarwinds ./reference/appid

# From the repository root, run non-benchmark Python tests
cd ..
python -m pytest -q -m "not benchmark"
```

> Note: `go test ./...` still includes experimental vendor/scoring/reference
> packages that are being remediated toward a green full-repo baseline.

```go
# Import platform code
import "github.com/mattpainter701/graphlayer/platform/orchestration/orchestration"
import "github.com/mattpainter701/graphlayer/platform/vendors/cisco/ios"
```

### Graph Engine Direction

Rust is required for graph construction, validation, algorithms and high-volume
exports. The shipped investigation engine uses Rust; older graph prototypes
must be ported before entering a user-facing workflow. See
[`docs/graph_rust_migration_plan.md`](docs/graph_rust_migration_plan.md).

## 📊 Sample Data

This repository also contains sample data from various vendors and tools:

### Network Configurations
- **Cisco/**: IOS, NX-OS, ASA configurations
- **PaloAlto/**: PAN-OS XML configurations
- **Fortinet/**: FortiGate configurations
- **Juniper/**: Junos configurations
- **sample_data/**: Additional test configurations and command outputs

### Asset/Vulnerability Data
- **RunZero/**: Network asset discovery data
- **Claroty/**: Industrial control system asset data
- **ForeScout/**: Endpoint visibility data
- **Nozomi/**: OT network monitoring data
- **Tenable/**: Vulnerability scan data

## 🔧 Platform Development

For platform development and integration:

1. **Read the documentation**: [`platform/docs/`](platform/docs/)
2. **Review vendor guides**: [`platform/vendors/*/README.md`](platform/vendors/)
3. **Check examples**: [`platform/orchestration/examples/`](platform/orchestration/examples/)
4. **Run tests**: `go test ./platform/...`

## 📝 Recent Changes

- **2025-10-31**: Reorganized into master `platform/` directory
- **2025-10-30**: Completed Phase 4 FortiGate/FortiManager support
- **2025-10-28**: Phase 1.5 L2 Security controls in progress

See [`platform/docs/project-history/`](platform/docs/project-history/) for complete history.

## 🤝 Contributing

When contributing to the platform:
1. Follow the existing directory structure
2. Add vendor code to `platform/vendors/{vendor}/`
3. Update relevant README files
4. Add comprehensive tests
5. Ensure vendor-agnostic model output

## 📞 Support

- **Platform Documentation**: [`platform/docs/getting-started/`](platform/docs/getting-started/)
- **Vendor Guides**: [`platform/vendors/`](platform/vendors/)
- **Project History**: [`platform/docs/project-history/`](platform/docs/project-history/)

---

**Main Platform Code**: [`platform/`](platform/) 🚀

**Sample Data**: Various vendor directories

**Documentation**: [`platform/docs/`](platform/docs/)
