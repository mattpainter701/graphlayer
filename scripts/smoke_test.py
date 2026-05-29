"""Quick smoke test for the platform connectors module."""
import importlib
import sys

def main():
    modules_to_test = [
        "platform.connectors.base_connector",
        "platform.connectors.fileshare_connector",
        "platform.connectors.netbox_connector",
        "platform.connectors.registry",
        "platform.connectors.servicenow_cmdb_connector",
        "platform.connectors.solarwinds_ncm_connector",
    ]
    failures = []
    for mod_name in modules_to_test:
        try:
            importlib.import_module(mod_name)
            print(f"OK: {mod_name}")
        except Exception as e:
            failures.append((mod_name, str(e)))
            print(f"FAIL: {mod_name}: {e}")

    # Also test top-level platform package (stdlib proxy)
    try:
        import platform
        print("OK: platform (stdlib proxy)")
    except Exception as e:
        failures.append(("platform", str(e)))

    # Registry functions
    try:
        from platform.connectors import get_connector, list_connectors, discover_connectors
        print("OK: registry functions imported")
    except Exception as e:
        failures.append(("registry functions", str(e)))

    if failures:
        print(f"\n{len(failures)} failure(s):")
        for name, reason in failures:
            print(f"  - {name}: {reason}")
        sys.exit(1)

    print("\nAll smoke tests passed!")

if __name__ == "__main__":
    main()
