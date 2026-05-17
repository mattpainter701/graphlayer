# Check Point Vendor Implementation

This directory contains parsers and models for Check Point network security devices.

## Supported Platforms

- **Gaia OS** - Check Point's unified operating system
- **CLISH** - Command Line Interface SHell
- **Security Policy** - Firewall rule base parsing

## Components

### Parsers

- `parsers/gaia_parser.go` - Parses Gaia configuration and CLISH output
- `parsers/security_policy_parser.go` - Parses security policies and normalizes to unified format

### Models

- `models/models.go` - Check Point specific data models

### Features

- ✅ Gaia configuration parsing (interfaces, routes, DNS, NTP, SNMP, users, services)
- ✅ CLISH command output parsing
- ✅ Security policy parsing with rule ordering awareness
- ✅ Unified SecurityPolicy output normalization
- ✅ Multi-vendor policy merging
- ✅ Rule ordering preservation

## Usage
