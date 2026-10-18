# Actual released workspace-v6 compatibility fixture

These five synthetic records were written by the actual released Store at
`d6d11db4ce7a411b013a6030af0e4d4a8d358028` (PR #17), using Go 1.25.0 on
the hosted Linux runner. They include four native ASA/PAN-OS/FortiOS snapshots
and one branching exploration with asymmetric configured return paths.

Capture provenance: [Test & Coverage run 39167133488](https://github.com/mattpainter701/graphlayer/actions/runs/39167133488),
artifact `released-v6-workspace`. The manual reproduction helper is
`e2e/capture-released-v6.js`. No record was synthesized by the v7 serializer.

`TestReleasedV6BytesSurviveWorkspaceV7Transactions` verifies replay and exact
preservation of these bytes after workspace/site/set publication and reopening.
Checksums detect corruption; they do not authenticate device origin. No real
customer configuration or credentials are present.
