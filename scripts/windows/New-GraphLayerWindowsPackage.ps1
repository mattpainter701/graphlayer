[CmdletBinding()]
param(
    [Parameter(Mandatory)] [ValidateSet('Development', 'Signed')] [string] $PackageKind,
    [Parameter(Mandatory)] [string] $InputDirectory,
    [Parameter(Mandatory)] [string] $OutputDirectory,
    [ValidateSet('CertificateStore', 'PreSigned')] [string] $SigningMode = 'CertificateStore',
    [string] $SignToolPath,
    [string] $CertificateThumbprint,
    [ValidateSet('CurrentUser', 'LocalMachine')] [string] $CertificateStore = 'CurrentUser',
    [string] $TimestampServer,
    [Parameter(Mandatory)] [ValidatePattern('^[0-9a-fA-F]{40}$')] [string] $SourceRevision
)

$ErrorActionPreference = 'Stop'
$scriptRoot = Split-Path -Parent $MyInvocation.MyCommand.Path
Import-Module (Join-Path $scriptRoot 'GraphLayer.WindowsPackage.psm1') -Force

$output = [IO.Path]::GetFullPath($OutputDirectory)
if (Test-Path -LiteralPath $output) { throw "Output directory already exists: $output" }
$inputDirectoryItem = Get-Item -LiteralPath $InputDirectory -Force
if (-not $inputDirectoryItem.PSIsContainer) { throw 'InputDirectory must be a directory.' }
if ($inputDirectoryItem.Attributes -band [IO.FileAttributes]::ReparsePoint) { throw 'InputDirectory must not be a reparse point.' }
$inputRoot = (Resolve-Path -LiteralPath $InputDirectory).Path
if ($PackageKind -eq 'Signed') {
    if ([string]::IsNullOrWhiteSpace($CertificateThumbprint) -or [string]::IsNullOrWhiteSpace($SignToolPath)) {
        throw 'Signed packages require SignToolPath and CertificateThumbprint.'
    }
    Assert-GraphLayerSignTool -Path $SignToolPath
    if ($SigningMode -eq 'CertificateStore') {
        if ([string]::IsNullOrWhiteSpace($TimestampServer)) { throw 'CertificateStore signing requires TimestampServer.' }
        $timestampUri = $null
        if (-not [Uri]::TryCreate($TimestampServer, [UriKind]::Absolute, [ref]$timestampUri) -or $timestampUri.Scheme -ne 'https') {
            throw 'TimestampServer must be an absolute HTTPS RFC 3161 timestamp URL.'
        }
        $cert = Get-GraphLayerSigningCertificate -Thumbprint $CertificateThumbprint -Store $CertificateStore
        $signerThumbprint = $cert.Thumbprint
    } else {
        $signerThumbprint = ($CertificateThumbprint -replace '\s', '').ToUpperInvariant()
        if ($signerThumbprint -notmatch '^[0-9A-F]{40}$') { throw 'CertificateThumbprint must be an exact 40-digit SHA1 certificate thumbprint.' }
        $cert = $null
    }
    $inputFiles = @(
        (Join-Path $inputRoot 'graphlayer-investigate.exe'),
        (Join-Path $inputRoot 'graphlayer-engine.exe')
    )
    foreach ($source in $inputFiles) {
        if (-not (Test-Path -LiteralPath $source -PathType Leaf)) { throw "Package input is missing: $source" }
        if ((Get-Item -LiteralPath $source -Force).Attributes -band [IO.FileAttributes]::ReparsePoint) { throw "Package input must not be a reparse point: $(Split-Path -Leaf $source)." }
        if (-not (Test-GraphLayerPE -Path $source)) { throw "Package input is not a valid PE executable: $source" }
        if ($SigningMode -eq 'PreSigned') {
            Assert-GraphLayerSignedFile -Path $source -ExpectedThumbprint $signerThumbprint -SignToolPath $SignToolPath -TimestampRequired
        }
    }
} else {
    $cert = $null
    $signerThumbprint = $null
}

$applicationPath = Join-Path $inputRoot 'graphlayer-investigate.exe'
$workerPath = Join-Path $inputRoot 'graphlayer-engine.exe'
foreach ($source in @($applicationPath, $workerPath)) {
    if (-not (Test-Path -LiteralPath $source -PathType Leaf)) { throw "Package input is missing: $source" }
    if ((Get-Item -LiteralPath $source -Force).Attributes -band [IO.FileAttributes]::ReparsePoint) { throw "Package input must not be a reparse point: $(Split-Path -Leaf $source)." }
    if (-not (Test-GraphLayerPE -Path $source)) { throw "Package input is not a valid PE executable: $source" }
}
if ($PackageKind -eq 'Signed' -and -not (Test-Path -LiteralPath $SignToolPath -PathType Leaf)) { throw "SignTool was not found: $SignToolPath" }

$parent = Split-Path -Parent $output
if (-not (Test-Path -LiteralPath $parent)) { New-Item -ItemType Directory -Path $parent | Out-Null }
$createdOutput = New-Item -ItemType Directory -Path $output
$createdOutputPath = [IO.Path]::GetFullPath($createdOutput.FullName)
try {
    $app = Join-Path $output 'graphlayer-investigate.exe'
    $worker = Join-Path $output 'graphlayer-engine.exe'
    Copy-Item -LiteralPath $applicationPath -Destination $app
    Copy-Item -LiteralPath $workerPath -Destination $worker

    if ($PackageKind -eq 'Signed' -and $SigningMode -eq 'CertificateStore') {
        $storeArg = if ($CertificateStore -eq 'LocalMachine') { '/sm' } else { $null }
        foreach ($file in @($app, $worker)) {
            $arguments = @('sign', '/sha1', $cert.Thumbprint, '/s', 'My')
            if ($storeArg) { $arguments += $storeArg }
            $arguments += @('/fd', 'SHA256', '/tr', $TimestampServer, '/td', 'SHA256', $file)
            & $SignToolPath @arguments
            if ($LASTEXITCODE -ne 0) { throw "SignTool failed for $(Split-Path -Leaf $file) with exit code $LASTEXITCODE." }
        }
    }
    if ($PackageKind -eq 'Signed') {
        Assert-GraphLayerSignedFile -Path $app -ExpectedThumbprint $signerThumbprint -SignToolPath $SignToolPath -TimestampRequired
        Assert-GraphLayerSignedFile -Path $worker -ExpectedThumbprint $signerThumbprint -SignToolPath $SignToolPath -TimestampRequired
    }

    $entries = @()
    foreach ($file in @($app, $worker)) {
        $entries += [ordered]@{ name = (Split-Path -Leaf $file); sha256 = (Get-FileHash -LiteralPath $file -Algorithm SHA256).Hash.ToLowerInvariant() }
    }
    $manifest = [ordered]@{
        schema = 'graphlayer-windows-package-v1'
        package_kind = $PackageKind
        signing_mode = if ($PackageKind -eq 'Signed') { $SigningMode } else { $null }
        commit = $SourceRevision.ToLowerInvariant()
        signer_thumbprint = $signerThumbprint
        files = $entries
    }
    $json = $manifest | ConvertTo-Json -Depth 5
    [IO.File]::WriteAllText((Join-Path $output 'manifest.json'), $json + "`n", [Text.UTF8Encoding]::new($false))
    $sums = ($entries | ForEach-Object { "$($_.sha256)  $($_.name)" }) -join "`n"
    [IO.File]::WriteAllText((Join-Path $output 'SHA256SUMS'), $sums + "`n", [Text.UTF8Encoding]::new($false))
    Write-Output $output
} catch {
    if (Test-Path -LiteralPath $createdOutputPath -PathType Container) {
        $resolvedOutput = [IO.Path]::GetFullPath((Resolve-Path -LiteralPath $createdOutputPath).Path)
        $currentOutput = Get-Item -LiteralPath $resolvedOutput -Force
        if ($resolvedOutput -eq $createdOutputPath -and -not ($currentOutput.Attributes -band [IO.FileAttributes]::ReparsePoint)) {
            Remove-Item -LiteralPath $createdOutputPath -Recurse -Force -ErrorAction SilentlyContinue
        }
    }
    throw
}
