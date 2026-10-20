[CmdletBinding()]
param(
    [Parameter(Mandatory)] [string] $PackageDirectory,
    [switch] $RequireSigned,
    [Alias('CertificateThumbprint')] [string] $ExpectedSignerThumbprint,
    [string] $SignToolPath
)

$ErrorActionPreference = 'Stop'
$scriptRoot = Split-Path -Parent $MyInvocation.MyCommand.Path
Import-Module (Join-Path $scriptRoot 'GraphLayer.WindowsPackage.psm1') -Force
$directory = [IO.Path]::GetFullPath($PackageDirectory)
$packageRoot = Get-Item -LiteralPath $directory -Force
if (-not $packageRoot.PSIsContainer) { throw 'PackageDirectory must be a directory.' }
if ($packageRoot.Attributes -band [IO.FileAttributes]::ReparsePoint) { throw 'Package directory must not be a reparse point.' }
$expectedNames = @('graphlayer-engine.exe', 'graphlayer-investigate.exe')
$expectedPayloads = @('LICENSE.txt', 'THIRD-PARTY-NOTICES.txt', 'graphlayer-engine.exe', 'graphlayer-investigate.exe')
$expectedEntries = @('LICENSE.txt', 'SHA256SUMS', 'THIRD-PARTY-NOTICES.txt', 'graphlayer-engine.exe', 'graphlayer-investigate.exe', 'manifest.json')
$packageItems = @(Get-ChildItem -LiteralPath $directory -Force)
$actualEntries = @($packageItems | ForEach-Object Name | Sort-Object)
if (($actualEntries -join '|') -ne (@($expectedEntries | Sort-Object) -join '|')) {
    throw 'Package directory must contain exactly four payloads, manifest.json, and SHA256SUMS; nested or extra entries are forbidden.'
}
foreach ($item in $packageItems) {
    if ($item.PSIsContainer -or ($item.Attributes -band [IO.FileAttributes]::ReparsePoint)) { throw "Package entry must be a regular file: $($item.Name)." }
}
$manifestPath = Join-Path $directory 'manifest.json'
if (-not (Test-Path -LiteralPath $manifestPath -PathType Leaf)) { throw 'Package manifest is missing.' }
$manifest = Get-Content -LiteralPath $manifestPath -Raw | ConvertFrom-Json
if ($manifest.schema -ne 'graphlayer-windows-package-v1') { throw 'Unsupported package manifest schema.' }
$expectedKind = if ($RequireSigned) { 'Signed' } else { 'Development' }
if ($manifest.package_kind -ne $expectedKind) { throw "Package kind mismatch: expected $expectedKind." }
if ($RequireSigned -and $manifest.signing_mode -notin @('CertificateStore', 'PreSigned')) { throw 'Signed package manifest has an invalid signing mode.' }
if (-not $RequireSigned -and $manifest.signing_mode) { throw 'Development package manifest must not claim a signing mode.' }
if ($manifest.commit -notmatch '^[0-9a-fA-F]{40}$') { throw 'Manifest source revision must be a 40-digit commit SHA.' }
$entries = @($manifest.files)
if ($entries.Count -ne 4 -or (@($entries | ForEach-Object name | Sort-Object) -join '|') -cne (@($expectedPayloads | Sort-Object) -join '|')) {
    throw 'Manifest must contain exactly the two executables and both license notices.'
}
$actualExe = @(Get-ChildItem -LiteralPath $directory -Filter '*.exe' -File | ForEach-Object Name | Sort-Object)
if (($actualExe -join '|') -ne (@($expectedNames | Sort-Object) -join '|')) { throw 'Package must contain exactly the application and Rust worker executables.' }
foreach ($entry in $entries) {
    $file = Join-Path $directory $entry.name
    if ((Get-Item -LiteralPath $file -Force).Attributes -band [IO.FileAttributes]::ReparsePoint) { throw "Package file must not be a reparse point: $($entry.name)." }
    if ($entry.name -in @('LICENSE.txt', 'THIRD-PARTY-NOTICES.txt') -and (Get-Item -LiteralPath $file -Force).Length -le 0) { throw "License notice must not be empty: $($entry.name)." }
    if ($entry.name -in $expectedNames -and -not (Test-GraphLayerPE -Path $file)) { throw "Package file is not a valid PE executable: $($entry.name)." }
    $actualHash = (Get-FileHash -LiteralPath $file -Algorithm SHA256).Hash.ToLowerInvariant()
    if ($entry.sha256 -notmatch '^[0-9a-f]{64}$' -or $actualHash -ne $entry.sha256) { throw "SHA-256 mismatch for $($entry.name)." }
}
$sumPath = Join-Path $directory 'SHA256SUMS'
if (-not (Test-Path -LiteralPath $sumPath -PathType Leaf)) { throw 'SHA256SUMS is missing.' }
$expectedSums = @($entries | ForEach-Object { "$($_.sha256)  $($_.name)" } | Sort-Object)
$actualSums = @(Get-Content -LiteralPath $sumPath | Where-Object { $_ -ne '' } | Sort-Object)
if (($actualSums -join "`n") -ne ($expectedSums -join "`n")) { throw 'SHA256SUMS does not match the package manifest.' }
if ($RequireSigned) {
    if ([string]::IsNullOrWhiteSpace($ExpectedSignerThumbprint)) { throw 'Signed package validation requires ExpectedSignerThumbprint.' }
    if ([string]::IsNullOrWhiteSpace($SignToolPath) -or -not (Test-Path -LiteralPath $SignToolPath -PathType Leaf)) { throw 'Signed package validation requires an existing SignToolPath.' }
    Assert-GraphLayerSignTool -Path $SignToolPath
    $expected = ($ExpectedSignerThumbprint -replace '\s', '').ToUpperInvariant()
    if ($expected -notmatch '^[0-9A-F]{40}$') { throw 'ExpectedSignerThumbprint must be an exact 40-digit SHA1 certificate thumbprint.' }
    if ($manifest.signer_thumbprint -ne $expected) { throw 'Manifest signer does not match the pinned signer thumbprint.' }
    foreach ($name in $expectedNames) {
        Assert-GraphLayerSignedFile -Path (Join-Path $directory $name) -ExpectedThumbprint $expected -SignToolPath $SignToolPath -TimestampRequired
    }
} elseif ($manifest.signer_thumbprint) {
    throw 'Development package manifest must not claim a release signer.'
}
Write-Output "Valid $expectedKind Windows package: $directory"
