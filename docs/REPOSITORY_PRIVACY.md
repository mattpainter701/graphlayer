# Repository privacy

Private configuration collections belong outside the source checkout. Use only
reviewed synthetic inputs in public tests, demonstrations and CI artifacts.
Saved workspaces, collection reports, customer names and raw configuration text
must not be committed or copied into pull requests or CI logs.

Run the repository guard before committing:

```text
python scripts/check-repository-privacy.py
python scripts/check-repository-privacy.py --history --private-corpus /path/outside/checkout
```

The first check examines tracked working files and the staged Git index,
including bounded gzip contents, for protected vocabulary fingerprints and
prohibited data paths. It reads staged blob bytes directly, so checkout filters
or unstaged edits cannot hide what a commit would contain.
The second also compares exact local corpus hashes with tracked bytes and Git
LFS payload pointers. The corpus is read locally and is never uploaded. Reports
contain aggregate counts only and must be written outside every Git worktree.
The protected vocabulary is stored as digests so the policy does not reproduce
the prohibited wording. With `--history`, the guard also examines the current
revision's complete ancestry, including deleted files and commit metadata. It
requires full history. CI fetches only that revision's ancestry and repeats the
tree/index/history check without customer data; unrelated remote branches are not
fetched for this job.

Opaque Git LFS pointers are rejected because their payload cannot be reviewed
by this check. Keep configuration collections and large data exports outside
Git; use small, reviewed synthetic fixtures as ordinary text.

This is a prevention check, not proof that arbitrary data is safe to publish.
Edited or transformed copies may no longer match an input hash. Review all new
fixtures and data exports before staging them; never point hosted CI at a
private corpus or supply it as an artifact.

Deleting a file in a new commit does not remove earlier versions. A historical
exposure needs a separate audit of branches, tags, pull-request references, Git
objects and hosted LFS payloads. Prepare and validate any rewrite in an isolated
copy; coordinate ref updates and other clones, and use the hosting provider's
sensitive-data process for cached references and orphaned LFS objects. A clean
working tree alone must not be reported as a complete historical purge.

The namespace cleanup changes legacy Go import paths to the project's
`github.com/mattpainter701/graphlayer/platform` modules and uses neutral
`Reference` comparison names. Compatibility aliases containing prohibited
wording are deliberately absent. Released investigation fixtures and saved
workspace formats remain unchanged. Exact, expiring Rust-boundary exceptions
cover source-only namespace edits; new domain algorithms still require Rust.
