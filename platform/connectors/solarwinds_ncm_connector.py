"""
SolarWinds NCM (Network Configuration Manager) Connector

This connector integrates with SolarWinds Orion NCM instances to download device
configurations via the Orion SDK (SWIS - SolarWinds Information Service).

Features:
- Connects to SolarWinds Orion NCM via SWIS API
- Supports both HTTP and HTTPS connections
- Retrieves device configurations from Cirrus.ConfigArchive
- Downloads running, startup, and custom config types
- Filters configs by device selection
- Saves configs with standardized naming: <hostname>_<config_type>.cfg

Dependencies:
- orionsdk (install via: pip install orionsdk)

Configuration:
- host: SolarWinds Orion server hostname/IP
- username: Orion username
- password: Orion password
- devices: List of device names/IDs to filter (optional)
- config_types: List of config types to download (default: ['Running'])
- use_https: Whether to use HTTPS (default: True)
- verify_ssl: Whether to verify SSL certificates (default: True)

Example Usage:
    connector_data = {
        "name": "SolarWinds NCM Production",
        "host": "solarwinds.example.com",
        "credentials": {...},
        "connector_type": "solarwinds_ncm",
        "devices": ["switch-core-01", "router-edge-01"],
        "payload": {
            "config_types": ["Running", "Startup"],
            "use_https": True,
            "verify_ssl": False
        }
    }
"""

import os
import re
from datetime import datetime

try:
    from orionsdk import SwisClient
except ImportError:
    raise ImportError("orionsdk library required. Install with: pip install orionsdk")

from graphlayer_utils import journal as logger

# Import the base connector class
# Adjust import based on your project structure
from .base_connector import BaseConnector


class SolarWindsNCMConnector(BaseConnector):
    """
    Connector for SolarWinds Orion NCM (Network Configuration Manager).

    Uses the Orion SDK (SWIS) to query and download device configurations
    from NCM's ConfigArchive.
    """

    SUPPORTS_DEVICES = True

    def __init__(self, guid: str, connector_data: dict, working_folder: str = None, task_guid: str = None):
        super().__init__(guid, connector_data, working_folder, task_guid)

        # Extract SolarWinds-specific configuration
        payload = connector_data.get("payload", {})

        # Config types to download (default: Running configs only)
        self.config_types = payload.get("config_types", ["Running"])

        # Connection settings
        self.use_https = payload.get("use_https", True)
        self.verify_ssl = payload.get("verify_ssl", True)

        # Download all devices if no filter specified
        self.download_all = len(self.devices) == 0

        # SWIS client (initialized in connect())
        self._swis = None

        logger.info(
            f"SolarWinds NCM connector initialized",
            extra={
                "connector_name": self.name,
                "host": self.host,
                "config_types": self.config_types,
                "device_filter": "all" if self.download_all else f"{len(self.devices)} devices",
                "use_https": self.use_https
            }
        )

    def connect(self):
        """Establish connection to SolarWinds Orion via SWIS API."""
        try:
            protocol = "https" if self.use_https else "http"
            server_url = f"{protocol}://{self.host}:17778"

            logger.debug(
                f"Connecting to SolarWinds Orion SWIS",
                extra={
                    "server": server_url,
                    "username": self.username,
                    "verify_ssl": self.verify_ssl
                }
            )

            # Create SWIS client
            self._swis = SwisClient(
                self.host,
                self.username,
                self.password,
                verify=self.verify_ssl
            )

            # Test connection with simple query
            test_query = "SELECT TOP 1 EngineID FROM Orion.Engines"
            result = self._swis.query(test_query)

            if result and "results" in result:
                logger.info(
                    f"Successfully connected to SolarWinds Orion",
                    extra={
                        "server": server_url,
                        "engine_count": len(result["results"])
                    }
                )
                return True
            else:
                raise ConnectionError("Failed to validate SWIS connection")

        except Exception as e:
            logger.error(
                f"Failed to connect to SolarWinds Orion",
                extra={
                    "host": self.host,
                    "error": str(e)
                }
            )
            raise ConnectionError(f"SolarWinds NCM connection failed: {e}") from e

    def disconnect(self):
        """Disconnect from SolarWinds Orion."""
        if self._swis:
            # SWIS client uses requests under the hood; no explicit disconnect needed
            self._swis = None
            logger.debug("Disconnected from SolarWinds Orion")

    def do_download(self, **kwargs):
        """
        Download device configurations from SolarWinds NCM.

        Process:
        1. Query Orion.Nodes to get device list
        2. Filter devices based on self.devices (if specified)
        3. Map Orion.Nodes.NodeID to Cirrus.Nodes.CoreNodeID
        4. Query Cirrus.ConfigArchive for latest configs
        5. Download and save config files

        Returns:
            dict: Summary of download operation
        """
        if not self._swis:
            raise RuntimeError("Not connected to SolarWinds. Call connect() first.")

        logger.info("Starting SolarWinds NCM config download")

        try:
            # Step 1: Get device list from Orion
            devices = self._get_device_list()

            if not devices:
                logger.warning("No devices found matching criteria")
                return {
                    "status": "completed",
                    "devices_processed": 0,
                    "configs_downloaded": 0,
                    "message": "No devices found"
                }

            logger.info(f"Found {len(devices)} devices to process")

            # Step 2: Download configs for each device and config type
            downloaded_count = 0

            for device in devices:
                for config_type in self.config_types:
                    try:
                        config_data = self._download_device_config(device, config_type)

                        if config_data:
                            self._save_config_file(device, config_type, config_data)
                            downloaded_count += 1

                    except Exception as e:
                        logger.error(
                            f"Failed to download {config_type} config for device",
                            extra={
                                "device": device.get("Caption", "Unknown"),
                                "config_type": config_type,
                                "error": str(e)
                            }
                        )

            logger.info(
                f"SolarWinds NCM download completed",
                extra={
                    "devices_processed": len(devices),
                    "configs_downloaded": downloaded_count
                }
            )

            return {
                "status": "completed",
                "devices_processed": len(devices),
                "configs_downloaded": downloaded_count,
                "message": f"Downloaded {downloaded_count} configs from {len(devices)} devices"
            }

        except Exception as e:
            logger.error(f"SolarWinds NCM download failed: {e}")
            raise

    def _get_device_list(self):
        """
        Query Orion to get list of devices.

        Returns:
            list: Device records with NodeID, Caption, and other metadata
        """
        # Build SWQL query to get devices
        query = """
            SELECT
                N.NodeID,
                N.Caption,
                N.DNS,
                N.IPAddress,
                N.Vendor,
                N.MachineType,
                CN.NodeID AS CirrusNodeID
            FROM Orion.Nodes N
            INNER JOIN Cirrus.Nodes CN ON N.NodeID = CN.CoreNodeID
            WHERE N.ObjectSubType = 'SNMP'
        """

        # Add device filter if specified
        if not self.download_all:
            # Escape single quotes in device names
            escaped_devices = ["'" + d.replace("'", "''") + "'" for d in self.devices]
            device_filter = ", ".join(escaped_devices)
            query += f" AND N.Caption IN ({device_filter})"

        logger.debug(f"Executing device query: {query}")

        result = self._swis.query(query)

        if result and "results" in result:
            devices = result["results"]
            logger.debug(f"Found {len(devices)} devices")
            return devices

        return []

    def _download_device_config(self, device, config_type):
        """
        Download a specific config type for a device from NCM ConfigArchive.

        Args:
            device (dict): Device record with CirrusNodeID
            config_type (str): Config type (e.g., 'Running', 'Startup')

        Returns:
            str: Configuration text, or None if not found
        """
        cirrus_node_id = device.get("CirrusNodeID")
        device_name = device.get("Caption", "Unknown")

        if not cirrus_node_id:
            logger.warning(f"Device {device_name} has no CirrusNodeID - skipping")
            return None

        # Query ConfigArchive for latest config
        query = f"""
            SELECT TOP 1
                ConfigID,
                NodeID,
                ConfigType,
                Config,
                DownloadTime
            FROM Cirrus.ConfigArchive
            WHERE NodeID = {cirrus_node_id}
              AND ConfigType = '{config_type}'
            ORDER BY DownloadTime DESC
        """

        logger.debug(
            f"Downloading {config_type} config for device",
            extra={
                "device": device_name,
                "cirrus_node_id": cirrus_node_id
            }
        )

        result = self._swis.query(query)

        if result and "results" in result and len(result["results"]) > 0:
            config_record = result["results"][0]
            config_text = config_record.get("Config", "")
            download_time = config_record.get("DownloadTime", "Unknown")

            logger.debug(
                f"Retrieved {config_type} config",
                extra={
                    "device": device_name,
                    "config_length": len(config_text),
                    "download_time": download_time
                }
            )

            return config_text

        logger.warning(
            f"No {config_type} config found in archive",
            extra={
                "device": device_name,
                "cirrus_node_id": cirrus_node_id
            }
        )

        return None

    def _save_config_file(self, device, config_type, config_data):
        """
        Save configuration to a file in the working folder.

        File naming: <hostname>_<config_type>.cfg

        Args:
            device (dict): Device record
            config_type (str): Config type
            config_data (str): Configuration text
        """
        hostname = device.get("Caption", "unknown_device")

        # Sanitize hostname for filesystem
        safe_hostname = self._sanitize_filename(hostname)

        # Create filename
        filename = f"{safe_hostname}_{config_type.lower()}.cfg"
        filepath = os.path.join(self.working_folder, filename)

        # Write config to file
        try:
            with open(filepath, 'w', encoding='utf-8') as f:
                # Add metadata header
                f.write(f"! Configuration downloaded from SolarWinds NCM\n")
                f.write(f"! Device: {hostname}\n")
                f.write(f"! IP Address: {device.get('IPAddress', 'N/A')}\n")
                f.write(f"! Vendor: {device.get('Vendor', 'N/A')}\n")
                f.write(f"! Machine Type: {device.get('MachineType', 'N/A')}\n")
                f.write(f"! Config Type: {config_type}\n")
                f.write(f"! Downloaded: {datetime.now().isoformat()}\n")
                f.write(f"!\n")
                f.write(config_data)

            logger.info(
                f"Saved config file",
                extra={
                    "device": hostname,
                    "config_type": config_type,
                    "filename": filename,
                    "size_bytes": len(config_data)
                }
            )

        except Exception as e:
            logger.error(
                f"Failed to save config file",
                extra={
                    "device": hostname,
                    "filename": filename,
                    "error": str(e)
                }
            )
            raise

    def _sanitize_filename(self, name):
        """
        Sanitize a string for use as a filename.

        Removes or replaces characters that are invalid in filenames.

        Args:
            name (str): Original name

        Returns:
            str: Sanitized name safe for filesystem
        """
        # Replace spaces with underscores
        name = name.replace(" ", "_")

        # Remove invalid filename characters
        name = re.sub(r'[<>:"/\\|?*]', '', name)

        # Limit length
        if len(name) > 200:
            name = name[:200]

        return name

    def trigger_ncm_download(self, node_ids, config_type="Running"):
        """
        Trigger NCM to download fresh configs from devices.

        This is useful if you want to force NCM to retrieve the latest configs
        before querying ConfigArchive.

        Args:
            node_ids (list): List of Cirrus NodeIDs
            config_type (str): Config type to download

        Returns:
            list: Transfer IDs for tracking download jobs
        """
        if not self._swis:
            raise RuntimeError("Not connected to SolarWinds")

        logger.info(
            f"Triggering NCM config download",
            extra={
                "node_count": len(node_ids),
                "config_type": config_type
            }
        )

        try:
            # Invoke DownloadConfig verb on Cirrus.ConfigArchive
            transfer_ids = self._swis.invoke(
                'Cirrus.ConfigArchive',
                'DownloadConfig',
                node_ids,
                config_type
            )

            logger.info(
                f"NCM download triggered",
                extra={
                    "transfer_ids": transfer_ids
                }
            )

            return transfer_ids

        except Exception as e:
            logger.error(f"Failed to trigger NCM download: {e}")
            raise


# Convenience function for testing
def test_connection(host, username, password, use_https=True, verify_ssl=False):
    """
    Test connection to SolarWinds Orion without full connector setup.

    Args:
        host (str): SolarWinds server hostname
        username (str): Orion username
        password (str): Orion password
        use_https (bool): Use HTTPS
        verify_ssl (bool): Verify SSL certificates

    Returns:
        dict: Connection test results
    """
    try:
        from orionsdk import SwisClient

        swis = SwisClient(host, username, password, verify=verify_ssl)

        # Test query
        result = swis.query("SELECT TOP 5 NodeID, Caption FROM Orion.Nodes")

        if result and "results" in result:
            return {
                "status": "success",
                "message": f"Connected successfully. Found {len(result['results'])} nodes.",
                "sample_nodes": [n.get("Caption") for n in result["results"]]
            }
        else:
            return {
                "status": "error",
                "message": "Connection succeeded but query returned no results"
            }

    except Exception as e:
        return {
            "status": "error",
            "message": f"Connection failed: {str(e)}"
        }
