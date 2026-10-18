# Embedded reference data

This directory does not contain a full IANA or IEEE registry snapshot. The
CSV files are header-only schema placeholders required by Go's `embed` rule;
they intentionally make no claim that the complete registries are available.

`NewIANALoader` uses the checked-in common protocol constants, Cisco's built-in
service defaults, and a bounded EtherType fallback for IPv4 (2048), ARP
(2054), and IPv6 (34525). Other protocol numbers and EtherTypes remain
unknown unless an explicitly supplied external catalog is loaded. Keep
unknown lookups unknown; do not infer a registry entry from a number alone.
