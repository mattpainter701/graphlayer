#!/bin/sh
set -eu
: "${GRAPHLAYER_TEST_PASSWORD:?An ephemeral test password is required}"
printf 'netfixture:%s\n' "$GRAPHLAYER_TEST_PASSWORD" | chpasswd
printf '%s\n%s\n' "$GRAPHLAYER_TEST_PASSWORD" "$GRAPHLAYER_TEST_PASSWORD" | smbpasswd -s -a netfixture >/dev/null
ssh-keygen -A >/dev/null 2>&1
cat > /etc/ssh/sshd_config <<'SSH'
Port 22
ListenAddress 0.0.0.0
HostKey /etc/ssh/ssh_host_ed25519_key
PasswordAuthentication yes
PermitRootLogin no
UsePAM no
AllowUsers netfixture
Subsystem sftp internal-sftp
LogLevel ERROR
SSH
cat > /etc/samba/smb.conf <<'SMB'
[global]
  workgroup = GRAPHFIXTURE
  server role = standalone server
  server min protocol = SMB2
  map to guest = Never
  log level = 0
  disable netbios = yes
  smb ports = 445
[configs]
  path = /srv/configs
  read only = yes
  valid users = netfixture
SMB
/usr/sbin/sshd
exec /usr/sbin/smbd --foreground --no-process-group
