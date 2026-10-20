# SignPath Foundation onboarding preparation

**Status: preparation only.** The project owner selected the free SignPath Foundation route and authorized preparation of the release workflow and documentation. No SignPath application has been submitted, and no SignPath organization, project, signing policy, certificate identity, or approval has been established. This document records facts and future setup steps; it does not accept provider terms or create an account.

## Project information for a future application

- Project: GraphLayer
- Source repository: <https://github.com/mattpainter701/graphlayer>
- Project license: Apache-2.0 is committed for GraphLayer source. Third-party components retain their respective licenses; inventory the exact Windows release and build dependencies before applying and confirm that all included components meet the Foundation's terms.
- Windows package notices: the release build generates `LICENSE.txt` and `THIRD-PARTY-NOTICES.txt` from the pinned Windows Rust/Go dependency reports, checked-in D3 license, and upstream toolchain attribution files. The collector fails closed on unknown or unreviewed dependency license identifiers, missing attribution/text, and Go source-sharing material. Inspect the generated output for the exact proposed release before making any application; the inventory is factual preparation and does not establish provider eligibility.
- Released Windows form: none is identified in the current release record. Current Windows CI is a development package. Publish and document the intended Windows release form before asking SignPath to sign it.
- Project description: GraphLayer imports network-device configuration and operational state, presents topology views, and evaluates configured firewall policies and network paths. Operators can import files or explicitly initiate supported connector acquisition. Results are configuration-derived investigation evidence; they do not establish live packet reachability.

Before an application, review the current Windows product behavior, every network transfer and third-party service, local workspace contents, installation/removal steps, and the exact files in the proposed release. Publish an accurate privacy statement and functionality/download page based on that review.

## Eligibility question

GraphLayer's Windows application is a passive configuration and path investigation tool. Its outputs explain policy decisions and configured paths from operator-supplied or explicitly acquired device data. SignPath Foundation's published terms exclude certain security diagnosis tools, including tools designed to identify exploitable vulnerabilities or actively scan for and highlight exploitable weaknesses. The terms do not determine from this description alone whether GraphLayer is eligible. The application should describe the product accurately and obtain SignPath's eligibility decision before relying on Foundation signing. No acceptance or rejection is implied here.

## Foundation requirements to prepare

The current Foundation terms describe these requirements for its free signing certificate:

- The project must use an OSI-approved license without commercial dual licensing; the project may not include proprietary code in the signed product. Review bundled components and their license terms.
- The project must be actively maintained, already released in the form to be signed, and documented on its download page.
- The team that owns and maintains the source must sign only its own binaries built from source it maintains.
- Team members must use MFA for GitHub and SignPath. Assign and publish Authors, Reviewers, and release Approvers. Each release signing request requires a human approval.
- Publish a **Code signing policy** on the project homepage and Windows download/release page. Include the required attribution, actual role assignments, and an accurate privacy statement or privacy-policy link.
- Configure SignPath metadata restrictions so both executables carry product name `GraphLayer` and matching product and file versions. Build artifacts from reviewed repository source in a verifiable way.

The Foundation issues its certificate to SignPath Foundation and retains discretion over acceptance and continued use. Read the current terms during onboarding; this preparation is not a substitute for them.

## Prepared source workflow

The opt-in [Windows signing workflow](../.github/workflows/sign-windows.yml) is a manual release path that only runs for this public repository on `main`. It requires a numeric `vX.Y.Z` tag pointing at the exact workflow SHA, requires the existing CI checks to succeed at that SHA, and builds both executables from source on a fresh GitHub-hosted Windows runner without reusing binary build caches. It checks product metadata before upload, submits the exact Actions artifact through SignPath, checks the returned executables and signatures, rebuilds the package hashes after signing, probes the actual sibling worker, and smoke-tests the packaged app. Its output is a review artifact; it does not publish a GitHub release.

The workflow fails before signing unless the `windows-signing` environment has its explicit `SIGNPATH_APPROVED` switch, configured identifiers, a token, and the expected certificate thumbprint. After provider onboarding, also configure GitHub environment reviewers and branch restrictions, SignPath GitHub.com as the trusted build system, positive origin verification, a release-only signing policy, and manual approval for every signing request. GitHub environment controls and SignPath approval are separate gates.

The [artifact configuration draft](../config/signpath/graphlayer-windows.xml) expects a ZIP containing exactly `graphlayer-engine.exe` and `graphlayer-investigate.exe`; it restricts both to the GraphLayer product name and the supplied version, then applies Authenticode signing. The final package also contains `LICENSE.txt` and `THIRD-PARTY-NOTICES.txt`; those files are added and hashed after the two executable files return from signing. Confirm the artifact configuration against the accepted SignPath project's generated schema/configuration before use.

| GitHub environment setting | Planned name | Purpose |
| --- | --- | --- |
| Secret | `SIGNPATH_API_TOKEN` | Token for a SignPath identity allowed to submit requests for this project and policy |
| Variable | `SIGNPATH_APPROVED` | Exact value `true` only after onboarding and release use are enabled |
| Variable | `SIGNPATH_ORGANIZATION_ID` | Accepted SignPath organization ID |
| Variable | `SIGNPATH_PROJECT_SLUG` | Configured project slug |
| Variable | `SIGNPATH_SIGNING_POLICY_SLUG` | Manually approved release policy slug |
| Variable | `SIGNPATH_ARTIFACT_CONFIGURATION_SLUG` | Versioned configuration for the Windows executable pair |
| Variable | `SIGNPATH_CERTIFICATE_THUMBPRINT` | Provider-confirmed signer thumbprint pinned for returned-file verification |

These names are placeholders until the service is approved and configured. The API token is never a repository variable or source file. The SignPath GitHub App, if installed for audit-log evaluation, is distinct from the scoped API token.

## Future application checklist

- [ ] Publish a Windows release in the exact form intended for signing and document its functionality and download.
- [ ] Review licenses and ownership for all shipped binaries, dependencies, and build scripts; confirm there is no proprietary or dual-licensed component that conflicts with the Foundation terms.
- [ ] Complete the product privacy, network-transfer, workspace-data, and installation/removal review; publish accurate documentation.
- [ ] Describe the configuration and path analysis accurately and obtain the Foundation's eligibility decision for its security-tool terms.
- [ ] Assign Authors, Reviewers, and release Approvers; enable MFA for every team member's GitHub and SignPath access.
- [ ] Publish the required Code signing policy and attribution on the project homepage and Windows download/release page.
- [ ] Read the current terms and submit a complete application using verified project facts.
- [ ] After acceptance, configure the trusted GitHub build system, project, metadata-restricted artifact configuration, release policy, origin verification, and manual release approvals.
- [ ] Configure protected GitHub environment reviewers and the listed environment secret/variables; verify only approved release refs can request signing.
- [ ] Validate a real approved signing request, returned signatures, package hashes, worker launch, and application smoke test before describing signing as operational.

## Official references

- [SignPath Foundation terms and eligibility](https://signpath.org/terms)
- [Apply for a free subscription](https://signpath.org/apply.html)
- [SignPath GitHub trusted build system and Actions integration](https://docs.signpath.io/trusted-build-systems/github)
- [SignPath project and signing policy setup](https://docs.signpath.io/projects)
- [Artifact configuration syntax](https://docs.signpath.io/artifact-configuration/syntax)
- [Metadata restriction examples](https://docs.signpath.io/artifact-configuration/examples)
- [Origin verification](https://docs.signpath.io/origin-verification/)
