#!/usr/bin/env python3
"""
Palo Alto App-ID Cache Downloader

This script downloads Palo Alto Networks App-ID data and builds a JSON cache
for offline lookups. The cache is used by the Go appid package for mapping
App-ID applications to traditional port/protocol services.

Usage:
    python download_appid.py --output palo_alto_app_id.json

Data Source:
    - Palo Alto Applipedia API (if available)
    - Or manually curated from Applipedia website

Note: This is a TEMPLATE script. You'll need to:
1. Obtain Palo Alto API credentials OR
2. Manually curate App-ID data from Applipedia website OR
3. Extract from PAN-OS firewall using XML API

Output Format:
{
    "version": "8595-8012",
    "last_updated": "2025-11-01",
    "source": "Applipedia",
    "applications": {
        "ssl": {
            "name": "ssl",
            "category": "networking",
            "subcategory": "encrypted-tunnel",
            "technology": "client-server",
            "description": "Secure Sockets Layer",
            "risk": 2,
            "default_ports": ["443"],
            "default_protocol": "tcp",
            ...
        },
        ...
    }
}
"""

import argparse
import json
import sys
from datetime import datetime


# Sample/minimal App-ID database
# In production, this would be downloaded from Applipedia or PAN-OS firewall
SAMPLE_APPID_DATABASE = {
    "version": "minimal-sample",
    "last_updated": datetime.now().isoformat(),
    "source": "Manual/Sample",
    "applications": {
        "ssl": {
            "name": "ssl",
            "category": "networking",
            "subcategory": "encrypted-tunnel",
            "technology": "client-server",
            "description": "Secure Sockets Layer protocol",
            "risk": 2,
            "default_ports": ["443"],
            "default_protocol": "tcp",
            "used_by_malware": False,
            "file_type_ident": False,
            "virus_ident": False,
            "data_ident": False,
            "deprecated": False,
            "parent_app": ""
        },
        "web-browsing": {
            "name": "web-browsing",
            "category": "general-internet",
            "subcategory": "internet-utility",
            "technology": "client-server",
            "description": "General web browsing traffic (HTTP/HTTPS)",
            "risk": 3,
            "default_ports": ["80", "443"],
            "default_protocol": "tcp",
            "used_by_malware": True,
            "file_type_ident": True,
            "virus_ident": True,
            "data_ident": True,
            "deprecated": False,
            "parent_app": ""
        },
        "dns": {
            "name": "dns",
            "category": "networking",
            "subcategory": "ip-protocol",
            "technology": "client-server",
            "description": "Domain Name System",
            "risk": 1,
            "default_ports": ["53"],
            "default_protocol": "udp",
            "used_by_malware": True,
            "file_type_ident": False,
            "virus_ident": False,
            "data_ident": False,
            "deprecated": False,
            "parent_app": ""
        },
        "ssh": {
            "name": "ssh",
            "category": "networking",
            "subcategory": "remote-access",
            "technology": "client-server",
            "description": "Secure Shell remote access protocol",
            "risk": 3,
            "default_ports": ["22"],
            "default_protocol": "tcp",
            "used_by_malware": True,
            "file_type_ident": False,
            "virus_ident": False,
            "data_ident": False,
            "deprecated": False,
            "parent_app": ""
        },
        "ftp": {
            "name": "ftp",
            "category": "business-systems",
            "subcategory": "general-business",
            "technology": "client-server",
            "description": "File Transfer Protocol",
            "risk": 3,
            "default_ports": ["21"],
            "default_protocol": "tcp",
            "used_by_malware": True,
            "file_type_ident": True,
            "virus_ident": True,
            "data_ident": False,
            "deprecated": False,
            "parent_app": ""
        },
        "smtp": {
            "name": "smtp",
            "category": "collaboration",
            "subcategory": "email",
            "technology": "client-server",
            "description": "Simple Mail Transfer Protocol",
            "risk": 2,
            "default_ports": ["25", "587"],
            "default_protocol": "tcp",
            "used_by_malware": True,
            "file_type_ident": True,
            "virus_ident": True,
            "data_ident": True,
            "deprecated": False,
            "parent_app": ""
        },
        "ping": {
            "name": "ping",
            "category": "networking",
            "subcategory": "ip-protocol",
            "technology": "network-protocol",
            "description": "ICMP Echo Request/Reply",
            "risk": 1,
            "default_ports": [],
            "default_protocol": "icmp",
            "used_by_malware": False,
            "file_type_ident": False,
            "virus_ident": False,
            "data_ident": False,
            "deprecated": False,
            "parent_app": ""
        },
        "rdp": {
            "name": "rdp",
            "category": "networking",
            "subcategory": "remote-access",
            "technology": "client-server",
            "description": "Microsoft Remote Desktop Protocol",
            "risk": 4,
            "default_ports": ["3389"],
            "default_protocol": "tcp",
            "used_by_malware": True,
            "file_type_ident": False,
            "virus_ident": False,
            "data_ident": False,
            "deprecated": False,
            "parent_app": ""
        },
        "vmware-vmotion": {
            "name": "vmware-vmotion",
            "category": "business-systems",
            "subcategory": "virtualization",
            "technology": "client-server",
            "description": "VMware vMotion live migration",
            "risk": 2,
            "default_ports": ["8000"],
            "default_protocol": "tcp",
            "used_by_malware": False,
            "file_type_ident": False,
            "virus_ident": False,
            "data_ident": False,
            "deprecated": False,
            "parent_app": ""
        },
        "ldap": {
            "name": "ldap",
            "category": "networking",
            "subcategory": "authentication",
            "technology": "client-server",
            "description": "Lightweight Directory Access Protocol",
            "risk": 2,
            "default_ports": ["389", "636"],
            "default_protocol": "tcp",
            "used_by_malware": False,
            "file_type_ident": False,
            "virus_ident": False,
            "data_ident": False,
            "deprecated": False,
            "parent_app": ""
        }
    }
}


def download_from_applipedia():
    """
    Download App-ID database from Palo Alto Applipedia.

    This is a placeholder function. In production, you would:
    1. Use Palo Alto's API (if available)
    2. Scrape Applipedia website (respect robots.txt)
    3. Extract from PAN-OS firewall XML API

    Returns:
        dict: App-ID database in cache format
    """
    print("WARNING: Using sample App-ID database", file=sys.stderr)
    print("For production, download from Applipedia or extract from PAN-OS firewall", file=sys.stderr)
    return SAMPLE_APPID_DATABASE


def main():
    parser = argparse.ArgumentParser(
        description="Download and cache Palo Alto App-ID database"
    )
    parser.add_argument(
        "--output",
        "-o",
        default="palo_alto_app_id.json",
        help="Output JSON file path (default: palo_alto_app_id.json)"
    )
    parser.add_argument(
        "--minimal",
        action="store_true",
        help="Use minimal sample database (for testing)"
    )

    args = parser.parse_args()

    print("Downloading App-ID database...")

    if args.minimal:
        appid_data = SAMPLE_APPID_DATABASE
    else:
        appid_data = download_from_applipedia()

    print(f"Retrieved {len(appid_data['applications'])} applications")
    print(f"Version: {appid_data['version']}")
    print(f"Source: {appid_data['source']}")

    # Save to JSON
    with open(args.output, 'w', encoding='utf-8') as f:
        json.dump(appid_data, f, indent=2, ensure_ascii=False)

    print(f"\nApp-ID cache saved to: {args.output}")
    print(f"File size: {os.path.getsize(args.output)} bytes")

    # Print summary by category
    categories = {}
    for app in appid_data['applications'].values():
        cat = app.get('category', 'unknown')
        categories[cat] = categories.get(cat, 0) + 1

    print("\nApplications by category:")
    for cat, count in sorted(categories.items()):
        print(f"  {cat}: {count}")


if __name__ == "__main__":
    import os
    main()
