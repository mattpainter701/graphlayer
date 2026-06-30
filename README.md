# GraphLayer

GraphLayer is a multi-vendor network topology graph engine. It ingests network
device configurations and operational state, normalizes topology data, builds
graph representations, and exports them for visualization and comparison.

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

The platform supports multiple network vendors:

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

GraphLayer is evaluating a focused Rust graph-engine refactor for graph
construction, validation, algorithms, and high-volume exports. See
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
