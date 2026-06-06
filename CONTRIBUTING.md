# Contributing

Thank you for considering contributing to the **GraphLayer Platform**!  
This document focuses on **connector and parser** development. For the general
guidelines please also read the existing guides in `platform/docs/`.

## Directory Layout

```
graphlayer/
├── platform/                  # Go core (library + binaries)
│   ├── cmd/                   # CLI entrypoints
│   ├── connectors/            # Python connector library
│   ├── correlation/           # Correlation engine
│   ├── graph/                 # Graph data model & algorithms
│   ├── intelligence/          # Inference & explainability
│   ├── models/                # Data models
│   ├── reference/             # IANA & vendor reference data
│   ├── scoring/               # Scoring engine
│   └── vendors/               # Vendor-specific parsers
├── tests/                     # Python integration tests
├── scripts/                   # Build & CI helper scripts
│   ├── check_packaging.sh     # Package integrity checks
│   └── smoke_test.py          # Connector import smoke test
├── docs/                      # Additional documentation
├── docker-compose.yml         # Development stack
├── docker-compose.qa.yml      # QA/CI stack
├── Dockerfile.core            # Core runtime image
└── Dockerfile.connectors      # Connector worker image
```

## Development Setup

### Prerequisites
- Go 1.24+ (`go version`)
- Python 3.12+ (`python3 --version`)
- Docker & Docker Compose (for QA stack)

### Quick Start
```bash
# Clone the repo
git clone https://github.com/mattpainter701/graphlayer.git
cd graphlayer

# Run Go tests (platform core)
cd platform && go test ./...

# Run Python connector tests
cd .. && pip install pytest && pytest tests/ -v

# Build all Docker images
docker compose build
```

## How to Add a New Connector

1. Create `platform/connectors/<vendor>_connector.py`
2. Register it in `platform/connectors/registry.py`
3. Add import to `platform/connectors/__init__.py`
4. Write tests in `tests/test_<vendor>_connector.py`
5. Run `python -m pytest tests/ -v` to verify
6. Update `scripts/smoke_test.py` with the new module path

## How to Add a New Vendor Parser (Go)

1. Create `platform/vendors/<vendor>/parser.go`
2. Implement the `Parser` interface from `platform/vendors/shared/`
3. Add fuzz tests in `platform/vendors/<vendor>/fuzz_test.go`
4. Run `cd platform && go test ./vendors/<vendor>/...`

## Code Review & CI

All changes must pass:

| Check | Description |
|-------|-------------|
| `lint` | golangci-lint + static analysis |
| `test` | Go tests across supported versions |
| `python-test` | Python connector integration tests |
| `qa` | Full Docker QA stack smoke test |
| `bench` | Performance regression check |
| `coverage` | Minimum 70% code coverage |

## Release Checklist

1. All CI checks passing on `main`
2. Run `scripts/check_packaging.sh` to verify distribution integrity
3. Run `scripts/smoke_test.py` for import sanity
4. Tag release: `git tag vX.Y.Z && git push origin vX.Y.Z`
5. Build and push Docker images via CI
