#!/usr/bin/env bash
# Run as root only inside a newly provisioned, dedicated Ubuntu 24.04 CI distro.
set -euo pipefail
test "$(id -u)" = 0
. /etc/os-release
test "$ID" = ubuntu
test "$VERSION_ID" = 24.04
test -f /etc/graphlayer-ci-runtime
if mountpoint -q /mnt/c || mountpoint -q /mnt/f; then
  echo 'Refusing runner setup with Windows data drives mounted' >&2
  exit 1
fi
export DEBIAN_FRONTEND=noninteractive
apt-get update
apt-get install -y ca-certificates curl git jq sudo build-essential pkg-config \
  libssl-dev libicu74 liblttng-ust1t64 libkrb5-3 zlib1g libunwind8 \
  python3 python3-venv python3-pip bc unzip xz-utils
install -m 0755 -d /etc/apt/keyrings
curl --fail --silent --show-error --location https://download.docker.com/linux/ubuntu/gpg -o /etc/apt/keyrings/docker.asc
chmod a+r /etc/apt/keyrings/docker.asc
cat > /etc/apt/sources.list.d/docker.sources <<'DOCKER'
Types: deb
URIs: https://download.docker.com/linux/ubuntu
Suites: noble
Components: stable
Architectures: amd64
Signed-By: /etc/apt/keyrings/docker.asc
DOCKER
apt-get update
apt-get install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin
if ! id graphlayer >/dev/null 2>&1; then
  useradd --create-home --shell /bin/bash graphlayer
fi
usermod -aG docker graphlayer
# Playwright installs Chromium's system packages through sudo, as on hosted CI.
printf 'graphlayer ALL=(ALL) NOPASSWD: ALL\n' > /etc/sudoers.d/90-graphlayer-ci
chmod 0440 /etc/sudoers.d/90-graphlayer-ci
visudo -cf /etc/sudoers.d/90-graphlayer-ci
install -m 0755 -d -o graphlayer -g graphlayer /opt/graphlayer-runner /opt/hostedtoolcache /opt/graphlayer-tools
runner_version=2.338.0
runner_sha=af4b794c1bc41d73d40535e3fe092a39f9679cd8d965954c2aca25a05ca41d32
curl --fail --silent --show-error --location --retry 3 \
  "https://github.com/actions/runner/releases/download/v${runner_version}/actions-runner-linux-x64-${runner_version}.tar.gz" \
  -o /opt/graphlayer-tools/runner.tar.gz
printf '%s  %s\n' "$runner_sha" /opt/graphlayer-tools/runner.tar.gz | sha256sum --check
tar -xzf /opt/graphlayer-tools/runner.tar.gz -C /opt/graphlayer-runner
chown -R graphlayer:graphlayer /opt/graphlayer-runner
curl --fail --silent --show-error --location --retry 3 \
  https://static.rust-lang.org/rustup/dist/x86_64-unknown-linux-gnu/rustup-init -o /opt/graphlayer-tools/rustup-init
curl --fail --silent --show-error --location \
  https://static.rust-lang.org/rustup/dist/x86_64-unknown-linux-gnu/rustup-init.sha256 \
  -o /opt/graphlayer-tools/rustup-init.sha256
rustup_sha=$(cut -d ' ' -f 1 /opt/graphlayer-tools/rustup-init.sha256)
printf '%s  %s\n' "$rustup_sha" /opt/graphlayer-tools/rustup-init | sha256sum --check
chmod 0755 /opt/graphlayer-tools/rustup-init
sudo -u graphlayer -H /opt/graphlayer-tools/rustup-init -y --profile minimal \
  --default-toolchain 1.99.1 --component rustfmt --component clippy
sudo -u graphlayer -H python3 -m venv /opt/graphlayer-tools/venv
systemctl enable --now docker
sudo -u graphlayer -H docker info --format '{{.ServerVersion}}'
printf 'Dedicated Linux CI dependencies are ready. No runner is registered yet.\n'
