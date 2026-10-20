# SignPath Foundation onboarding draft

**Status: preparation only.** No application has been submitted, SignPath has not
approved GraphLayer, and no SignPath organization, project, signing policy,
certificate, or publisher relationship is established. This document does not
select a project license or accept SignPath's terms. The repository owner must
review and approve any application and terms before submission or signing.

## Application draft

- Project: GraphLayer
- Repository: <https://github.com/mattpainter701/graphlayer>
- Project homepage: [owner to confirm]
- Windows release/download page and existing released artifact: [owner to provide]
- License: [undecided; owner must select an OSI-approved license covering all
  project components, with no commercial dual licensing, before applying]

GraphLayer imports network-device configurations and operational state, builds
topology views, and evaluates firewall policies and configured network paths.
The Windows application is a local, single-user investigation tool. It can
identify and explain policy decisions and possible network paths from supplied
configuration data; it does not establish that packets traverse a live network.
The application handles sensitive network configuration and saved investigation
data. See the current [investigation guide](FIREWALL_INVESTIGATION.md) for its
documented handling and limitations. The owner must confirm the full product
scope, privacy and network behavior, third-party components, and exact Windows
release artifacts before applying.

**Eligibility question requiring an explicit SignPath decision:** GraphLayer
analyzes firewall configurations and reports policy and path outcomes. SignPath's
published terms exclude certain security diagnosis and hacking tools, including
tools designed to identify vulnerabilities or actively scan for and highlight
exploitable weaknesses. We have not determined whether SignPath considers
GraphLayer in scope for this exclusion. Describe this functionality accurately
in the application and ask SignPath to decide eligibility before integrating
Foundation signing. No eligibility or approval is implied here.

Proposed build and signing process, subject to acceptance: a GitHub Actions
workflow will run on GitHub-hosted Windows runners, build from a reviewed
`vX.Y.Z` tag that points to the exact green `main` workflow commit, and submit the two application
executables through SignPath's GitHub trusted build integration. Each release
signing request will require manual approval. The SignPath artifact
configuration draft is [`config/signpath/graphlayer-windows.xml`](../config/signpath/graphlayer-windows.xml).

## Eligibility and owner checklist

- [ ] Choose and apply an OSI-approved license to all project components; confirm
  there is no commercial dual licensing.
- [ ] Confirm the source, build scripts, and every file to be signed are
  maintained by the GraphLayer project, and review bundled third-party and
  proprietary components.
- [ ] Identify an already released Windows artifact in the form proposed for
  signing, and publish a page that describes its functionality and provides the
  download.
- [ ] Ask SignPath to assess GraphLayer's firewall and network security analysis
  against its security-tool exclusion; record its explicit eligibility answer.
- [ ] Confirm the product's complete privacy and network behavior, third-party
  service behavior, any system changes, and removal/uninstallation instructions.
- [ ] Decide who holds the Author, Reviewer, and release Approver responsibilities.
  Names and responsibilities have not been assigned in this draft.
- [ ] Ensure every team member uses MFA on GitHub and SignPath.
- [ ] Publish an approved Code signing policy on the project homepage and
  Windows release/download page, including the required attribution, actual
  role assignments, and a truthful privacy statement.
- [ ] Review and accept SignPath's current terms, then submit an application at
  <https://signpath.org/apply> with the verified project, release, license,
  ownership, privacy, and role details.
- [ ] Ensure both built PE files contain the required GraphLayer product name
  and the same release product/file version; the configuration restricts these
  values and does not add missing version resources.
- [ ] After approval, create/configure the SignPath organization integration,
  project, GitHub trusted build system, release signing policy, and this versioned
  artifact configuration; require positive origin verification, limit releases
  to the approved ref policy, and manually approve each release request.
- [ ] Validate the exact signed release artifact and both executable signatures
  on Windows under the intended policy before describing the signing workflow as
  operational.

SignPath's application form is web-rendered; the official page's readable text
does not expose its current field list. Prepare the facts above and enter only
verified information in the form. The official policy and integration references
are linked below.

## Planned GitHub Actions settings

These names are reserved for the planned release workflow. They are not evidence
that a SignPath account or configuration exists. Set them only after application
approval and project setup.

| GitHub location | Name | Intended value |
| --- | --- | --- |
| Environment `windows-signing` secret | `SIGNPATH_API_TOKEN` | API token for an identity with submitter permission for the configured project and policy |
| Environment `windows-signing` variable | `SIGNPATH_ORGANIZATION_ID` | SignPath organization ID |
| Environment `windows-signing` variable | `SIGNPATH_PROJECT_SLUG` | SignPath project slug |
| Environment `windows-signing` variable | `SIGNPATH_SIGNING_POLICY_SLUG` | Manually approved release signing policy slug |
| Environment `windows-signing` variable | `SIGNPATH_ARTIFACT_CONFIGURATION_SLUG` | `windows-amd64` |
| Environment `windows-signing` variable | `SIGNPATH_CERTIFICATE_THUMBPRINT` | Approved certificate thumbprint used to verify returned signatures |
| Environment `windows-signing` variable | `SIGNPATH_APPROVED` | Exact string `true`, set only after the owner approves using SignPath for a release |

The workflow is prepared for manual dispatch from `main` with a numeric `vX.Y.Z`
tag that resolves to the exact workflow commit, after all thirteen existing CI
checks pass at that SHA. All jobs leading to signing must
use GitHub-hosted runners. The signing job must upload an archive containing
exactly `graphlayer-engine.exe` and `graphlayer-investigate.exe`, then submit the
uploaded GitHub Actions artifact to SignPath. SignPath's GitHub Action requires
the API token, organization ID, project slug, signing policy slug, and uploaded
artifact ID; it accepts the artifact configuration slug and request parameters.
Pass the same tag version as the required `version` parameter in the XML
configuration. Restrict the release policy to the approved branch/tag rules and
enable positive origin verification. Install the SignPath GitHub App with access
to this repository for audit-log evaluation; this is separate from the scoped
API token used by the action.

The SignPath project must link the predefined GitHub.com trusted build system.
When trusted-build-system verification is enabled, interactive users cannot be
declared as signing-policy submitters. A CI identity with submitter permission
submits requests; a designated human Approver manually approves every release
request. Approval remains required even when the workflow is otherwise eligible.

## Product and user-data notes for owner review

The current Windows package is a portable bundle of the GraphLayer investigation
application and Rust worker, rather than a conventional installer. It creates or
opens a local workspace; users can remove the application files by deleting the
bundle. Workspace files hold network configuration evidence and saved
investigations, so documentation must tell users to preserve or back up data
they want to keep before deleting or moving it. This description must be checked
against the released package and current product behavior before it is used in an
application or public policy.

The investigation documentation says the browser UI runs locally on loopback
and that supplied configurations can contain sensitive network information. The
owner must confirm whether any release feature transmits user data or contacts
third-party services, and document applicable privacy behavior before making a
public privacy statement.

## Official SignPath references

- [Foundation eligibility, certificate conditions, policy-page requirements,
  roles, MFA, and manual approval](https://signpath.org/terms)
- [Apply for a free SignPath.io subscription](https://signpath.org/apply)
- [GitHub trusted build integration and action inputs](https://docs.signpath.io/trusted-build-systems/github)
- [Project, signing policy, trusted-build, and artifact-configuration setup](https://docs.signpath.io/projects)
- [Artifact configuration syntax and parameters](https://docs.signpath.io/artifact-configuration/syntax)
- [ZIP artifact and metadata restriction examples](https://docs.signpath.io/artifact-configuration/examples)
- [User and project role permissions](https://docs.signpath.io/users/)
