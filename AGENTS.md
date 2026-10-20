# Repository engineering policy

## Collaboration and Git ownership

- Default subagents to Luna with medium reasoning. Override only when the task
  requires it or the delegation tool does not support Luna.
- Use one branch and worktree per active task. Before edits, confirm owner,
  branch, HEAD and dirty state. Other active sessions' checkouts are immutable.
- Fetch/prune origin, inspect existing worktrees/branches/PRs and compare against
  current origin/main before starting or resuming. Create feature branches from
  current origin/main; never implement on main. Record ownership and starting
  SHA in task commentary. Coordinate overlapping files before edits.
- Recover stale branches by behavior and patch equivalence, not unique commit
  counts. Preserve current main unless recovered behavior is demonstrably better.
  Review idle worktrees after four hours, classify after 24 hours, and explicitly
  recover or retire work older than seven days. Never automatically delete dirty
  work or unreferenced detached commits.
- Product work must update against current origin/main, resolve semantically,
  recheck the final diff, add focused tests and customer release notes, complete
  one documentation-impact and one release-note option plus security attestation
  in the PR, push/open it in the same task, and await all required checks and the
  merge gate. Merge only green final-head CI, verify the exact merge on
  origin/main, then remove the completed clean worktree. Follow branch retention.
- End with worktree/branch/HEAD, cleanliness, pushed status, PR/check state,
  conflicts/untracked work, ownership and whether cleanup is safe. Substantial
  uncommitted work requires a clear handoff. Do not repurpose another task's
  worktree or discard its changes.

## Repository privacy

- Customer configuration collections, raw imports and private collection reports
  must remain outside every Git worktree. Only reviewed synthetic fixtures may
  be committed. Never publish corpus filenames, source text, credentials or
  unredacted parser/process output in Git, CI, pull requests or artifacts.
- Prohibited repository identities are represented by fingerprints in
  `config/repository-privacy.json`; do not put their wording in source, paths,
  test fixtures, documentation or commit messages.
- Before committing from a clean-history clone, run
  `python scripts/check-repository-privacy.py --history` and, when
  the local corpus is available, repeat with `--private-corpus` pointing outside
  the checkout. The command emits aggregate counts only. CI repeats the tracked
  tree, staged index and current-revision ancestry scan. Never push from an old contaminated
  clone. These checks do not prove cleanup of other refs or hosted LFS storage.
- A detected exposure requires a local audit of all refs, configuration-byte
  fingerprints and LFS pointers. Keep remediation reports outside Git. Do not
  claim a complete purge until published refs, hosted caches/LFS and affected
  clones have been addressed. Coordinate destructive history rewrites and
  preserve unrelated working files before changing repository history.

## Language boundary

- Rust owns domain behavior: vendor parsing and connectors, graph and topology
  construction, policy and route evaluation, NAT, network path analysis,
  validation, inference, and scaling algorithms.
- Implement a behavior in Rust before enabling it in a user-facing workflow.
  Go or Python wrappers may validate inputs, call the Rust implementation, and
  translate compatibility formats; they must not reimplement the behavior.
- JavaScript, TypeScript, and CSS may manage view state and render results. They
  must not parse network configurations or implement graph, policy, routing,
  or analysis rules.
- Existing Go HTTP, storage, and CLI compatibility code can remain during
  migration. Do not add new Go or Python domain implementations. Keep the
  remaining legacy inventory frozen, except for narrowly scoped security or
  correctness fixes and migration adapters recorded in
  `config/rust-boundary.json` with an owner, reason, and expiry no more than 90
  days away. Each exception pins the SHA256 of the exact approved file.
- Do not add production Go or Python files under `platform/`; implement new
  domain behavior in Rust. A temporary transport adapter needs an exact-path
  exception with its approved content hash.
- Port every vendor parser and Python connector. Legacy inventory is not a
  claim that Rust support is complete.

## Migration and verification

- Keep Rust as the source of truth. A facade that calls the old engine does
  not count as a Rust port.
- The local investigation application's ASA/PAN-OS/FortiOS parsers, model
  validation, policy/path/return/exploration engine and connector acquisition
  must execute in Rust. Preserve public Go/Python signatures only as transport
  and serialization adapters; remove the replaced production algorithms.
- Legacy experimental libraries are frozen, not supported extensions. Port and
  test their behavior in Rust before connecting them to a shipped workflow.
- Preserve input compatibility and verify behavior with golden fixtures and
  focused parity checks as each implementation moves.
- Never refresh released golden outputs from a replacement engine to hide a
  mismatch. Preserve workspace-v1 through v7 bytes, evidence IDs, source hashes,
  parser versions and saved investigation identities during replay.
- Missing workers, protocol errors and timeouts must fail closed without
  rewriting saved workspaces or selecting a Go/Python domain fallback.
- Run `python scripts/check-rust-boundary.py` after changing production code
  in a guarded domain area. CI runs the same command.
- Do not update legacy hashes to accept a change until the implementation has
  moved to Rust or the change has a documented, time-limited exception.
- Do not commit credentials, raw device configurations, or unredacted logs.
- Keep customer configuration corpora and their reports outside the checkout
  and hosted CI. Use `scripts/check-private-corpus.py` for local inventory and
  Rust validation; publish only explicitly reviewed synthetic fixtures.
