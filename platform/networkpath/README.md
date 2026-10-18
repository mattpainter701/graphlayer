# Multi-firewall configuration paths

`Evaluate(Input)` composes the bounded ASA `pathanalysis` model across explicit
snapshot-scoped nodes and operator-supplied direct links. It carries the actual
translated tuple at every boundary, checks static next hops against linked peer
interface addresses, and preserves independent forward and return dispositions.

Return legs assume an established TCP/UDP session admitted on the same forward
firewalls. `pathanalysis.EvaluateReturn` rechecks the supplied forward evidence,
uses that permit as the session assumption, and requires the exact reverse tuple,
NAT identities and interfaces. Configured return routes and reverse links must
agree. No live state or asymmetric state sharing is inferred.

Evidence is qualified by snapshot ID. The app persists the full request and
result and replays both directions on reopen. See the [operator and API
contract](../../docs/MULTI_FIREWALL_INVESTIGATIONS.md) for scope and limits.
