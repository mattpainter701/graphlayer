# Windows package signing and validation

## Current finding

The unsigned Rust worker was blocked by Smart App Control. The observed Code Integrity event 3077 names policy `VerifiedAndReputableDesktop`, with requested signing level 2 and validated signing level 1. Its correlated event 3089 reports publisher `Unknown`. Windows therefore treated this worker as unsigned for the active policy. This evidence points to a policy decision at process launch; it does not establish a Rust executable-format or runtime defect.

Keep the event's policy identity and signature details with the release investigation. Event 3077 includes the blocked file, hashes, policy name and ID, status, and requested and validated signing levels. Correlate it with 3089 by ActivityID; inspect each signature's VerificationError and publisher fields. Microsoft documents requested level 2 plus validated level 1 as a file treated as unsigned, which can mean it has no signature, its signature is invalid, or the policy does not trust its signer. The current evidence identifies `VerifiedAndReputableDesktop`, so investigate Smart App Control's trusted-signing path rather than assuming an arbitrary enterprise policy caused this block.

The existing unsigned CI bundle must be labeled `Development`. A successful packaging step or successful Authenticode verification alone does not complete the fix. Do not report the worker issue resolved until the signed package's worker launches successfully under the unchanged policy and the resulting Code Integrity evidence confirms the intended acceptance.

## Signing requirements

Use a code-signing identity approved for this application and this device. Smart App Control accepts code signed with a valid certificate issued by a CA in Microsoft's Trusted Root Program, as well as code Microsoft app intelligence identifies as safe. Enterprise App Control policies can impose additional signer rules; the organization's policy owner must approve the signing identity and any required policy trust through its normal process. A public certificate does not automatically become an approved signer in every enterprise policy.

The project ships no signing identity or private key. Do not substitute a self-signed or test certificate, export a private key, or place key material in the repository. For certificate-store signing, select the exact approved thumbprint in the specified `CurrentUser` or `LocalMachine` `My` store; the command requires a private key, Code Signing EKU, and current validity. Use the organization’s managed signing service, HSM, or approved certificate store for key custody.

A locally generated self-signed certificate does not establish the public trust required by Smart App Control. [SignPath Foundation](https://signpath.org/terms) offers free signing for eligible open-source projects; its conditions include an OSI-approved project license and eligibility review. GraphLayer's project license remains undecided, so eligibility is not established. [Microsoft Artifact Signing Public Trust](https://learn.microsoft.com/en-us/azure/artifact-signing/concept-trust-models) is another option, requiring an account, identity validation and an applicable service plan. Provisioning either identity requires the project owner's participation; package scripts do not create accounts, accept terms, or change Windows trust policy.

For a signing service that returns already signed binaries, use `-PackageKind Signed -SigningMode PreSigned` with the approved signing certificate thumbprint and SignTool path. This verifies both returned embedded signatures and timestamps before writing the package manifest and hashes; it does not import or require a local private key. Obtain the expected signer identity through the approved signing process rather than trusting a thumbprint supplied by an untrusted download.

The manifest records the source revision supplied by the build. It is not a signed build-provenance attestation; verify the CI build origin separately. Authenticode verification authenticates executable contents and signer identity, not the manifest's revision field.

For Smart App Control distribution, the signing certificate must chain to a CA in Microsoft's Trusted Root Program. If enterprise App Control signer rules also govern the device, confirm the identity and cryptographic algorithm meet those rules. Microsoft documents that App Control signer-based rules support RSA signatures; ECC signatures can fail with VerificationError 23. Smart App Control itself supports RSA and ECC, so use RSA when the same package must satisfy both Smart App Control and an RSA-only enterprise signer rule.

Sign every shipped executable component, including the application and worker. Use SHA-256 file digests and an RFC 3161 timestamp with SHA-256. Verify the completed files with SignTool using the Authenticode policy and verbose output, then validate the exact package by launching it under the same unchanged policy. A successful `signtool verify` validates the Authenticode signature against the local Windows trust state; it does not by itself prove an enterprise App Control policy accepts the file or that the application runs correctly. Offline verification can also be limited by unavailable chain or revocation data.

## Package workflow

The Windows package builder is planned as `scripts/windows/New-GraphLayerWindowsPackage.ps1`. Its current interface is:

```powershell
& .\scripts\windows\New-GraphLayerWindowsPackage.ps1 `
  -PackageKind Development `
  -InputDirectory <directory-containing-both-executables> `
  -OutputDirectory <new-directory> `
  -SourceRevision <source-revision>
```

For a signed package, require the approved certificate and timestamp service explicitly:

```powershell
& .\scripts\windows\New-GraphLayerWindowsPackage.ps1 `
  -PackageKind Signed `
  -InputDirectory <directory-containing-both-executables> `
  -OutputDirectory <new-directory> `
  -CertificateThumbprint <approved-sha1-thumbprint> `
  -CertificateStore CurrentUser `
  -SignToolPath <signtool.exe> `
  -TimestampServer <approved-rfc3161-url> `
  -SourceRevision <source-revision>
```

Use `LocalMachine` only when that is the approved certificate store. The builder rejects missing or ambiguous signing inputs, resolves the exact thumbprint only in the selected `My` store, signs both PE files, and verifies their signatures, signer thumbprint, and timestamps before writing the manifest and SHA-256 sums. Keep signed and development output directories distinct. Verify a signed package with:

```powershell
& .\scripts\windows\Test-GraphLayerWindowsPackage.ps1 `
  -PackageDirectory <package-directory> `
  -RequireSigned `
  -ExpectedSignerThumbprint <approved-sha1-thumbprint> `
  -SignToolPath <signtool.exe>
```

The package test checks the manifest, the exact application and worker files, recorded hashes, and—when `Signed` is required—both embedded signatures, timestamp presence, and exact signer thumbprint. Unexpected files, directories and reparse points are rejected.

Then probe the exact worker through its existing one-shot protocol:

```powershell
& .\scripts\windows\Test-GraphLayerWorker.ps1 `
  -PackageDirectory <package-directory> `
  -PackageKind Signed `
  -ExpectedSignerThumbprint <approved-sha1-thumbprint> `
  -SignToolPath <signtool.exe>
```

This bounded probe asks `path.is_asa_vendor` about `cisco-asa`, with no configuration or customer input. It verifies the package first, then directly launches its exact worker; there is no alternate engine or fallback. `-PackageKind Development` is reserved for explicitly unsigned development packages on hosts that permit them, including CI. It does not establish Smart App Control compatibility.

Also launch the exact packaged application under the unchanged Smart App Control policy, import the built-in mixed-vendor sample and complete the topology investigation workflow. For a block, correlate fresh 3077 and 3089 events and retain the policy ID, status, VerificationError, and signer details. A signed package is considered validated only after the worker starts successfully under that policy.

## Microsoft documentation

- [App Control debugging and troubleshooting](https://learn.microsoft.com/en-us/windows/security/application-security/application-control/app-control-for-business/operations/appcontrol-debugging-and-troubleshooting) explains 3077/3089 correlation, the signing-level fields, and checking active policies.
- [App Control event tag explanations](https://learn.microsoft.com/en-us/windows/security/application-security/application-control/windows-defender-application-control/operations/event-tag-explanations) defines signing levels and VerificationError values.
- [Smart App Control overview](https://learn.microsoft.com/en-us/windows/apps/develop/smart-app-control/overview) describes its trusted code and enforcement behavior.
- [Sign your app for Smart App Control](https://learn.microsoft.com/en-us/windows/apps/develop/smart-app-control/code-signing-for-smart-app-control) covers trusted certificates and signing options.
- [App Control file rule levels](https://learn.microsoft.com/en-us/windows/security/threat-protection/windows-defender-application-control/select-types-of-rules-to-create) describes publisher and certificate signer rules and their scope.
- [App Control known issues](https://learn.microsoft.com/en-us/windows/security/application-security/application-control/app-control-for-business/operations/known-issues) documents the RSA-only limitation for signer-based rules.
- [SignTool](https://learn.microsoft.com/en-us/windows/win32/seccrypto/signtool) documents signing and verification commands and digest options.
- [Verify a file signature with SignTool](https://learn.microsoft.com/en-us/windows/win32/seccrypto/using-signtool-to-verify-a-file-signature) documents `signtool verify /pa /v`.
- [Time-stamping Authenticode signatures](https://learn.microsoft.com/en-us/windows/win32/seccrypto/time-stamping-authenticode-signatures) recommends SHA-256 and RFC 3161 timestamps.
- [Artifact Signing trust models](https://learn.microsoft.com/en-us/azure/artifact-signing/concept-trust-models) distinguishes public trust from private and test certificate profiles.
