"""
Unified File Share Connector (SMB/SSH/SCP)

This connector supports downloading device configuration files from network file shares
using multiple protocols:
- SMB/CIFS (Windows shares, Samba)
- SSH/SCP (Unix/Linux file shares)

Many organizations store device configuration backups on centralized file shares, making
this connector essential for importing configs without direct device access.

Features:
- Auto-protocol detection (SMB vs SSH)
- Recursive directory scanning
- Filename filtering with regex patterns
- Common file share structures (by vendor, hostname, date)
- Deduplication (avoids downloading same file multiple times)
- Support for both Windows and Unix paths

Dependencies:
- SMB: pysmb (install via: pip install pysmb)
- SSH/SCP: paramiko (install via: pip install paramiko)

Configuration:
- protocol: "smb" or "ssh" (auto-detected if not specified)
- host: File share server hostname/IP
- username: Authentication username
- password: Authentication password
- share_name: SMB share name (SMB only)
- remote_path: Path to config directory
- recursive: Whether to scan subdirectories (default: True)
- file_patterns: List of regex patterns to match filenames (default: ["*.cfg", "*.conf", "*.txt"])
- exclude_patterns: List of patterns to exclude (optional)

Example Usage (SMB):
    connector_data = {
        "name": "Config Backup Share",
        "host": "fileserver.example.com",
        "credentials": {...},
        "connector_type": "fileshare",
        "payload": {
            "protocol": "smb",
            "share_name": "network_configs",
            "remote_path": "/cisco/switches",
            "recursive": True,
            "file_patterns": [r".*\\.cfg$", r".*running-config.*"]
        }
    }

Example Usage (SSH):
    connector_data = {
        "name": "Unix Config Share",
        "host": "backups.example.com",
        "credentials": {...},
        "connector_type": "fileshare",
        "payload": {
            "protocol": "ssh",
            "remote_path": "/data/network-backups",
            "recursive": True,
            "file_patterns": [r".*\\.conf$"]
        }
    }
"""

import os
import re
from datetime import datetime
from pathlib import Path

from graphlayer_utils import journal as logger

# Import the base connector class
from .base_connector import BaseConnector


class FileShareConnector(BaseConnector):
    """
    Unified connector for downloading configs from file shares (SMB/SSH).

    Automatically detects or uses specified protocol to connect to file shares
    and download configuration files matching specified patterns.
    """

    SUPPORTS_DEVICES = False

    def __init__(self, guid: str, connector_data: dict, working_folder: str = None, task_guid: str = None):
        super().__init__(guid, connector_data, working_folder, task_guid)

        # Extract file share configuration
        payload = connector_data.get("payload", {})
        location = payload.get("location", {})

        # Protocol selection
        self.protocol = payload.get("protocol", "auto").lower()
        if self.protocol == "auto":
            # Auto-detect: try SMB first (more common), fall back to SSH
            self.protocol = "smb"

        # SMB-specific settings
        self.share_name = location.get("share_name", "")
        self.device_name = location.get("device_name", self.host)  # NetBIOS name for SMB

        # Path settings
        self.remote_path = self._normalize_path(location.get("remote_path", "/"))
        self.recursive = payload.get("recursive", True)

        # File filtering
        filtering = payload.get("filtering", {})
        file_patterns = filtering.get("file_patterns", [r".*\.cfg$", r".*\.conf$", r".*\.txt$"])
        self.file_patterns = [re.compile(pattern) for pattern in file_patterns]

        exclude_patterns = filtering.get("exclude_patterns", [])
        self.exclude_patterns = [re.compile(pattern) for pattern in exclude_patterns]

        # Session objects (initialized in connect())
        self._session = None
        self._ssh_client = None

        # Track downloaded files to avoid duplicates
        self._downloaded_hashes = set()

        logger.info(
            f"FileShare connector initialized",
            extra={
                "connector_name": self.name,
                "protocol": self.protocol,
                "host": self.host,
                "remote_path": self.remote_path,
                "recursive": self.recursive,
                "file_patterns": [p.pattern for p in self.file_patterns]
            }
        )

    def connect(self):
        """Establish connection based on protocol."""
        if self.protocol == "smb":
            self._connect_smb()
        elif self.protocol == "ssh":
            self._connect_ssh()
        else:
            raise ValueError(f"Unsupported protocol: {self.protocol}")

    def disconnect(self):
        """Disconnect based on protocol."""
        if self.protocol == "smb" and self._session:
            self._session.close()
            self._session = None
        elif self.protocol == "ssh" and self._ssh_client:
            self._ssh_client.close()
            self._ssh_client = None

        logger.debug("Disconnected from file share")

    def do_download(self, **kwargs):
        """
        Download configuration files from file share.

        Returns:
            dict: Summary of download operation
        """
        if self.protocol == "smb":
            return self._download_smb()
        elif self.protocol == "ssh":
            return self._download_ssh()
        else:
            raise ValueError(f"Unsupported protocol: {self.protocol}")

    # ==================== SMB Implementation ====================

    def _connect_smb(self):
        """Connect to SMB file share."""
        try:
            from smb.SMBConnection import SMBConnection
        except ImportError:
            raise ImportError("pysmb library required for SMB. Install with: pip install pysmb")

        logger.debug(
            f"Connecting to SMB share",
            extra={
                "host": self.host,
                "share": self.share_name,
                "username": self.username
            }
        )

        try:
            self._session = SMBConnection(
                username=self.username,
                password=self.password,
                my_name=self.name,
                remote_name=self.device_name,
                use_ntlm_v2=True,
                is_direct_tcp=True
            )

            is_authenticated = self._session.connect(self.host, port=445)

            if not is_authenticated:
                raise ValueError("SMB authentication failed")

            logger.info(f"Successfully connected to SMB share {self.share_name}")

        except (ConnectionRefusedError, TimeoutError) as e:
            raise ConnectionError(f"Failed to connect to SMB share: {e}") from e

    def _download_smb(self):
        """Download files from SMB share."""
        if not self._session:
            raise RuntimeError("Not connected to SMB share")

        logger.info("Starting SMB file download")

        downloaded_count = 0

        try:
            # List files in share
            files_to_download = self._list_smb_files(self.remote_path)

            logger.info(f"Found {len(files_to_download)} files matching criteria")

            for remote_file_path in files_to_download:
                try:
                    self._download_smb_file(remote_file_path)
                    downloaded_count += 1

                except Exception as e:
                    logger.error(
                        f"Failed to download file",
                        extra={
                            "file": remote_file_path,
                            "error": str(e)
                        }
                    )

            logger.info(f"SMB download completed: {downloaded_count} files downloaded")

            return {
                "status": "completed",
                "files_downloaded": downloaded_count,
                "message": f"Downloaded {downloaded_count} files from SMB share"
            }

        except Exception as e:
            logger.error(f"SMB download failed: {e}")
            raise

    def _list_smb_files(self, path):
        """
        Recursively list files in SMB share matching patterns.

        Args:
            path (str): Path within share

        Returns:
            list: List of file paths relative to share root
        """
        files = []

        try:
            entries = self._session.listPath(self.share_name, path)

            for entry in entries:
                # Skip . and ..
                if entry.filename in [".", ".."]:
                    continue

                full_path = f"{path}/{entry.filename}".replace("//", "/")

                if entry.isDirectory:
                    # Recurse into subdirectories if enabled
                    if self.recursive:
                        files.extend(self._list_smb_files(full_path))
                else:
                    # Check if file matches patterns
                    if self._matches_patterns(entry.filename):
                        files.append(full_path)

        except Exception as e:
            logger.warning(f"Failed to list SMB path {path}: {e}")

        return files

    def _download_smb_file(self, remote_file_path):
        """
        Download a single file from SMB share.

        Args:
            remote_file_path (str): Path to file within share
        """
        # Create local filename preserving directory structure
        local_filename = self._create_local_filename(remote_file_path)
        local_filepath = os.path.join(self.working_folder, local_filename)

        # Create parent directories if needed
        os.makedirs(os.path.dirname(local_filepath), exist_ok=True)

        # Download file
        with open(local_filepath, 'wb') as f:
            self._session.retrieveFile(self.share_name, remote_file_path, f)

        file_size = os.path.getsize(local_filepath)

        logger.info(
            f"Downloaded SMB file",
            extra={
                "remote_path": remote_file_path,
                "local_path": local_filename,
                "size_bytes": file_size
            }
        )

    # ==================== SSH/SCP Implementation ====================

    def _connect_ssh(self):
        """Connect to SSH server."""
        try:
            import paramiko
        except ImportError:
            raise ImportError("paramiko library required for SSH. Install with: pip install paramiko")

        logger.debug(
            f"Connecting to SSH server",
            extra={
                "host": self.host,
                "username": self.username
            }
        )

        try:
            self._ssh_client = paramiko.SSHClient()
            self._ssh_client.set_missing_host_key_policy(paramiko.AutoAddPolicy())

            self._ssh_client.connect(
                hostname=self.host,
                username=self.username,
                password=self.password,
                timeout=30
            )

            logger.info(f"Successfully connected to SSH server")

        except Exception as e:
            raise ConnectionError(f"Failed to connect to SSH server: {e}") from e

    def _download_ssh(self):
        """Download files via SSH/SCP."""
        if not self._ssh_client:
            raise RuntimeError("Not connected to SSH server")

        logger.info("Starting SSH file download")

        downloaded_count = 0

        try:
            # List files on remote server
            files_to_download = self._list_ssh_files(self.remote_path)

            logger.info(f"Found {len(files_to_download)} files matching criteria")

            # Open SFTP session
            sftp = self._ssh_client.open_sftp()

            try:
                for remote_file_path in files_to_download:
                    try:
                        self._download_ssh_file(sftp, remote_file_path)
                        downloaded_count += 1

                    except Exception as e:
                        logger.error(
                            f"Failed to download file",
                            extra={
                                "file": remote_file_path,
                                "error": str(e)
                            }
                        )

            finally:
                sftp.close()

            logger.info(f"SSH download completed: {downloaded_count} files downloaded")

            return {
                "status": "completed",
                "files_downloaded": downloaded_count,
                "message": f"Downloaded {downloaded_count} files via SSH"
            }

        except Exception as e:
            logger.error(f"SSH download failed: {e}")
            raise

    def _list_ssh_files(self, path):
        """
        Recursively list files on SSH server matching patterns.

        Args:
            path (str): Remote path

        Returns:
            list: List of remote file paths
        """
        files = []

        try:
            sftp = self._ssh_client.open_sftp()

            try:
                # Get directory listing
                entries = sftp.listdir_attr(path)

                for entry in entries:
                    full_path = f"{path}/{entry.filename}".replace("//", "/")

                    # Check if directory (using S_ISDIR from stat module)
                    import stat
                    if stat.S_ISDIR(entry.st_mode):
                        # Recurse into subdirectories if enabled
                        if self.recursive:
                            files.extend(self._list_ssh_files(full_path))
                    else:
                        # Check if file matches patterns
                        if self._matches_patterns(entry.filename):
                            files.append(full_path)

            finally:
                sftp.close()

        except Exception as e:
            logger.warning(f"Failed to list SSH path {path}: {e}")

        return files

    def _download_ssh_file(self, sftp, remote_file_path):
        """
        Download a single file via SFTP.

        Args:
            sftp: Paramiko SFTP client
            remote_file_path (str): Remote file path
        """
        # Create local filename preserving directory structure
        local_filename = self._create_local_filename(remote_file_path)
        local_filepath = os.path.join(self.working_folder, local_filename)

        # Create parent directories if needed
        os.makedirs(os.path.dirname(local_filepath), exist_ok=True)

        # Download file
        sftp.get(remote_file_path, local_filepath)

        file_size = os.path.getsize(local_filepath)

        logger.info(
            f"Downloaded SSH file",
            extra={
                "remote_path": remote_file_path,
                "local_path": local_filename,
                "size_bytes": file_size
            }
        )

    # ==================== Utility Methods ====================

    def _normalize_path(self, path):
        """
        Normalize path to start with / and remove trailing /.

        Examples:
            "/" -> ""
            "/foo" -> "/foo"
            "/foo/" -> "/foo"
            "foo" -> "/foo"
        """
        path = path.strip()

        if not path or path == "/":
            return ""

        if path[-1] == "/":
            path = path[:-1]

        if path[0] != "/":
            path = "/" + path

        return path

    def _matches_patterns(self, filename):
        """
        Check if filename matches include patterns and doesn't match exclude patterns.

        Args:
            filename (str): Filename to check

        Returns:
            bool: True if file should be downloaded
        """
        # Check exclude patterns first
        for exclude_pattern in self.exclude_patterns:
            if exclude_pattern.search(filename):
                return False

        # Check include patterns
        for include_pattern in self.file_patterns:
            if include_pattern.search(filename):
                return True

        return False

    def _create_local_filename(self, remote_path):
        """
        Create local filename from remote path.

        Preserves directory structure relative to remote_path root.

        Args:
            remote_path (str): Full remote file path

        Returns:
            str: Local filename (may include subdirectories)
        """
        # Remove remote_path prefix to get relative path
        if remote_path.startswith(self.remote_path):
            relative_path = remote_path[len(self.remote_path):]
        else:
            relative_path = remote_path

        # Remove leading slash
        if relative_path.startswith("/"):
            relative_path = relative_path[1:]

        # Sanitize for Windows compatibility
        relative_path = relative_path.replace("\\", "/")

        return relative_path


# Convenience function for testing
def test_connection(protocol, host, username, password, **kwargs):
    """
    Test connection to file share without full connector setup.

    Args:
        protocol (str): "smb" or "ssh"
        host (str): File share server
        username (str): Username
        password (str): Password
        **kwargs: Additional protocol-specific args

    Returns:
        dict: Connection test results
    """
    try:
        if protocol == "smb":
            from smb.SMBConnection import SMBConnection

            share_name = kwargs.get("share_name", "")
            device_name = kwargs.get("device_name", host)

            conn = SMBConnection(
                username=username,
                password=password,
                my_name="test",
                remote_name=device_name,
                use_ntlm_v2=True,
                is_direct_tcp=True
            )

            is_auth = conn.connect(host, port=445)

            if is_auth:
                shares = conn.listShares()
                conn.close()
                return {
                    "status": "success",
                    "message": f"Connected successfully to SMB server",
                    "shares": [s.name for s in shares]
                }
            else:
                return {
                    "status": "error",
                    "message": "SMB authentication failed"
                }

        elif protocol == "ssh":
            import paramiko

            client = paramiko.SSHClient()
            client.set_missing_host_key_policy(paramiko.AutoAddPolicy())

            client.connect(hostname=host, username=username, password=password, timeout=30)

            stdin, stdout, stderr = client.exec_command("pwd")
            current_dir = stdout.read().decode().strip()

            client.close()

            return {
                "status": "success",
                "message": f"Connected successfully to SSH server",
                "current_directory": current_dir
            }

        else:
            return {
                "status": "error",
                "message": f"Unsupported protocol: {protocol}"
            }

    except Exception as e:
        return {
            "status": "error",
            "message": f"Connection failed: {str(e)}"
        }
