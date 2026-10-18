# Workspaces, sites and reviewed batch imports

GraphLayer's local workspace now organizes imported evidence into sites and named snapshot sets. A snapshot set pins the exact configurations used as an operational baseline. Existing policy, routing and multi-firewall investigations keep their immutable snapshot IDs and result contracts.

## Operator workflow

1. Name the local workspace and optionally create a site with a short description. The server's configured private data directory remains the workspace boundary. A site is an organizational label, not a separate account or authorization boundary.
2. Add native ASA, PAN-OS set-format or FortiOS CLI files to the batch queue. Review file names and detected vendor selection, or load the four-firewall sample through the normal parser pipeline.
3. Preview the batch. Every item reports validation state, parsed hostname/context, parser version, policy counts, warnings, capabilities and sanitized source lines. Preview does not publish snapshots or change the active set. The source excerpt is bounded and identifies its full line count; credentials and unsupported source values follow the existing parser sanitization rules.
4. Give the set a name, select a site or leave it unassigned, then publish. Every item must validate. Duplicate configurations within the batch must be removed; configurations already present in the workspace reuse their canonical immutable snapshots.
5. Inspect the active set's members or use it to populate a network investigation. Links are supplied explicitly by the operator. Imported interface and route facts do not establish device adjacency or live connectivity.
6. Open a different named set to change the active baseline. Saved investigations continue to replay their original snapshots. Workspace name, sites, sets and active selection survive a server restart.

The legacy single-configuration import and all saved investigation workflows remain available. Selecting a snapshot outside the active set is shown explicitly; switching an active baseline does not rewrite historical results.

## Model and persistence

The model separates the local workspace profile, sites, snapshot-set membership and immutable snapshots. Member source names belong to the set. Site labels and workspace names are excluded from snapshot identity, so renaming or grouping evidence cannot invalidate saved results or silently duplicate content.

Workspace-v7 adds immutable workspace transactions. One atomic, private record publishes a complete validated batch, its named set and the active-set selection. Metadata changes also append transactions. Reopen verifies the transaction chain, revisions, references, snapshot validation, quotas and checksums before exposing the state. An incomplete or invalid batch cannot partially activate a baseline. Checksums detect corruption and inconsistent records; they do not authenticate a device's origin or provide a signature against an attacker able to rewrite local files.

Released workspace-v1 through v6 files remain readable without byte rewriting. Existing parser and policy/path/network/exploration contracts remain unchanged. Back up a workspace before upgrading; older executables cannot open the new records.

## API and bounds

`GET /api/workspace` returns the profile and named sets. `PUT /api/workspace` renames it. `POST /api/sites` creates a site. `GET /api/snapshot-sets` and `GET /api/snapshot-sets/:id` expose set membership. `POST /api/snapshot-sets/:id/activate` selects a baseline. Mutations require the current `expected_revision` so stale clients cannot overwrite a newer selection.

`POST /api/import-previews` accepts `{items: [{name, config, vendor}]}`. It returns a short-lived, server-owned preview with per-file validation and sanitized excerpts. `POST /api/import-previews/:id/publish` accepts `{set_name, site_id, expected_revision}` and publishes the reviewed snapshots. Exact publication retries are idempotent. Changed names/sites, expired previews, unknown site IDs and stale revisions fail explicitly. A restart discards unpublished previews and requires a new review.

Batches contain 1–8 files and at most 2 MiB of total input configuration. The existing 4 MiB HTTP request and 8 MiB persisted record budgets apply. At most four previews live in memory for 15 minutes. Excerpts show up to 60 physical lines, each bounded to 512 bytes, and 20 warnings, with full counts and explicit truncation. Workspace limits remain 32 unique snapshots and 256 investigations, with up to 32 sites, 64 named sets and 256 workspace transactions. Size/quota/write failures leave the previous committed state intact.

This delivery supports local file collection. Network connectors, credential management, scheduled collection, enterprise identity and shared workspaces remain subsequent work; connector health is not fabricated by the shell.

## Verification

Hosted Linux and Windows tests exercise publication, revision conflicts, duplicate handling, failed writes, restart/reopen, detached results, transaction validation and compatibility with actual released records. HTTP tests cover strict JSON, same-origin session/CSRF checks, bounded previews, expiry, stale clients and idempotent publication.

Browser tests cover the complete workspace → site → mixed-vendor batch → sanitized review → named set → explicit network path → source evidence → saved replay/export journey. They also cover invalid batches, preview invalidation, baseline switching, mobile layout and keyboard operation. Existing investigation cases remain in the suite. Test servers use dedicated workspaces so the product's snapshot quota remains unchanged.

Normal executable/server launch is blocked by this task host's Windows application-control policy. Local build, vet and syntax checks remain available; runtime execution and browser verification run in hosted CI without changing that policy.
