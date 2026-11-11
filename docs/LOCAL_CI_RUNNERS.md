# Local CI runners

The five GitHub Actions workflows can route Linux jobs to the dedicated runner
when repository variable `GRAPHLAYER_ONPREM_RUNNERS` is `true`. Routing also
requires the repository to be private, its name to be exactly
`mattpainter701/graphlayer`, and the event to be a push to `main` or
`codex/repository-privacy` or the explicitly reviewed
`codex/windows-worker-release-recovery` branch. Pull requests, including fork pull requests, and
manual QA dispatches use GitHub-hosted runners. The Windows native job uses
`windows-latest`; it is deferred while private local validation is enabled to
avoid paid hosted execution, and resumes when the repository is public. A
deferred Windows job is not evidence of a successful native Windows check.

## Runner host

Use a dedicated Ubuntu 24.04.5 WSL distribution named `GraphLayerCI`, stored
outside every Git checkout. Verify its Ubuntu image against the official
[Ubuntu SHA256SUMS](https://releases.ubuntu.com/noble/SHA256SUMS).
The distribution's `/etc/wsl.conf` disables Windows drive automounting,
`fstab` mounting, interop, and Windows PATH inheritance, and enables systemd:

```ini
[automount]
enabled=false
mountFsTab=false

[interop]
enabled=false
appendWindowsPath=false

[boot]
systemd=true
```

These settings are scoped to this distribution; existing distributions and
global WSL configuration were left unchanged. Docker Engine is installed from
its official Ubuntu apt repository inside the distribution. Docker Desktop,
the Windows Docker host socket, and host bind mounts are not used.

## Bootstrap and job lifecycle

Use `scripts/ci/bootstrap-runner.sh` only in the dedicated Ubuntu 24.04
distribution. It requires the `/etc/graphlayer-ci-runtime` sentinel before it
installs runtime dependencies. The browser job installs Chromium's system
packages through Playwright. Bootstrap grants the `graphlayer` account Docker access and sudo inside
the distribution. The JIT runner starts with `scripts/ci/run-jit-runner.sh` as
that account; the short-lived encoded runner configuration is supplied on
standard input.

Use the official GitHub Actions runner package at version `2.338.0` and verify
it against the checksums published on the [official runner v2.338.0 release](https://github.com/actions/runner/releases/tag/v2.338.0).
Register it for this repository only, with labels `self-hosted`, `Linux`,
`X64`, and `graphlayer-private`, and use one job per registration.

One Linux runner process serializes all 12 Linux jobs and avoids collisions on
the shared Docker daemon, loopback ports, and fixed container names. The
dedicated WSL distribution persists between jobs and is not a VM sandbox. A
one-job JIT registration unregisters the runner after its job; it does not
erase the distribution or clean the job workspace. The runner controller must
clean workspace data and job credentials between jobs before registering the
next runner. Use `python scripts/ci/run-local-ci.py --head <reviewed-commit-sha>`
from the Windows checkout. The controller verifies the private repository,
exact branch revision and eligible job queue before each registration. Logs
stay under `%LOCALAPPDATA%/GraphLayer/runner-controller`, outside Git. Keep
customer corpora and their reports outside every runner
workspace and mount.

The controller defaults to `codex/repository-privacy`. After the coordinated
history cleanup, use `--branch main --head <reviewed-commit-sha>` to validate
private main pushes. For the reviewed Windows recovery, use
`--branch codex/windows-worker-release-recovery --head <reviewed-commit-sha>`.
It accepts only those three exact branch names and verifies the
selected branch and every workflow run against the exact reviewed revision.

## Public repository transition

Enable `GRAPHLAYER_ONPREM_RUNNERS` only while trusted maintainers control pushes
to the three enabled branches. Before making the repository public, disable the
variable and remove all repository runner registrations. Hosted jobs use the
configured GitHub artifact and cache storage. Local Linux jobs keep
artifacts under `/opt/graphlayer-ci-evidence/<commit>/<run>/<job>/` and build
caches under `/opt/graphlayer-ci-cache/`, rather than using GitHub
artifact or cache storage. Archive manifests record hashes and counts; they
do not publish the archived files. Preserve these local results before
retiring the runner environment.

The runner uses umask `077`. Container builds set executable permissions
explicitly for the non-root application user and readable permissions for
reviewed synthetic acquisition fixtures. Keep the runner's restrictive umask;
do not loosen workspace or credential permissions to accommodate a container.
