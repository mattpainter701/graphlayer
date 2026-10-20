#!/usr/bin/env bash
# Supply the short-lived encoded JIT configuration on standard input, never Git.
set -euo pipefail
umask 077
test "$(id -un)" = graphlayer
test -f /etc/graphlayer-ci-runtime
if mountpoint -q /mnt/c || mountpoint -q /mnt/f; then
  echo 'Refusing CI with Windows data drives mounted' >&2
  exit 1
fi
export PATH="/home/graphlayer/.cargo/bin:/opt/graphlayer-tools/venv/bin:$PATH"
export AGENT_TOOLSDIRECTORY=/opt/hostedtoolcache
export RUNNER_TOOL_CACHE=/opt/hostedtoolcache
export CARGO_BUILD_JOBS=4
export CARGO_TARGET_DIR=/opt/graphlayer-ci-cache/target
export GOMAXPROCS=4
cd /opt/graphlayer-runner
IFS= read -r jit_configuration
test -n "$jit_configuration"
exec ./run.sh --jitconfig "$jit_configuration"
