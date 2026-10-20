function Normalize-GraphLayerThumbprint([string] $Value) {
    return ($Value -replace '\s', '').ToUpperInvariant()
}

function Test-GraphLayerPE([string] $Path) {
    $stream = $null
    $reader = $null
    try {
        $stream = [IO.File]::OpenRead($Path)
        if ($stream.Length -lt 64) { return $false }
        $reader = [IO.BinaryReader]::new($stream)
        if ($reader.ReadUInt16() -ne 0x5A4D) { return $false }
        $stream.Position = 0x3c
        $offset = $reader.ReadInt32()
        if ($offset -lt 64 -or $offset -gt ($stream.Length - 4)) { return $false }
        $stream.Position = $offset
        return ($reader.ReadUInt32() -eq 0x00004550)
    } catch { return $false }
    finally {
        if ($reader) { $reader.Dispose() } elseif ($stream) { $stream.Dispose() }
    }
}

function Get-GraphLayerSigningCertificate([string] $Thumbprint, [string] $Store) {
    $wanted = Normalize-GraphLayerThumbprint $Thumbprint
    if ($wanted -notmatch '^[0-9A-F]{40}$') { throw 'SignerThumbprint must be an exact 40-digit SHA1 certificate thumbprint.' }
    $storePath = if ($Store -eq 'LocalMachine') { 'Cert:\LocalMachine\My' } else { 'Cert:\CurrentUser\My' }
    $matchingCertificates = @(Get-ChildItem -LiteralPath $storePath | Where-Object { (Normalize-GraphLayerThumbprint $_.Thumbprint) -eq $wanted })
    if ($matchingCertificates.Count -ne 1) { throw "Expected exactly one certificate with thumbprint $wanted in $storePath." }
    $cert = $matchingCertificates[0]
    $now = Get-Date
    if (-not $cert.HasPrivateKey) { throw "Selected certificate $wanted has no private key." }
    if ($now -lt $cert.NotBefore -or $now -gt $cert.NotAfter) { throw "Selected certificate $wanted is outside its validity period." }
    $codeSigningOid = '1.3.6.1.5.5.7.3.3'
    $eku = @($cert.Extensions | Where-Object { $_.Oid.Value -eq '2.5.29.37' } | ForEach-Object { $_.EnhancedKeyUsages } | ForEach-Object { $_.Value })
    if ($codeSigningOid -notin $eku) { throw "Selected certificate $wanted lacks the Code Signing EKU." }
    return $cert
}

function Assert-GraphLayerSignedFile {
    param([string] $Path, [string] $ExpectedThumbprint, [string] $SignToolPath, [switch] $TimestampRequired)
    $expected = Normalize-GraphLayerThumbprint $ExpectedThumbprint
    $signature = Get-AuthenticodeSignature -LiteralPath $Path
    if ($signature.Status -ne [Management.Automation.SignatureStatus]::Valid) {
        throw "Authenticode verification failed for $(Split-Path -Leaf $Path): $($signature.Status)."
    }
    if ($signature.SignatureType -ne 'Authenticode') {
        throw "An embedded Authenticode signature is required for $(Split-Path -Leaf $Path)."
    }
    if (-not $signature.SignerCertificate -or (Normalize-GraphLayerThumbprint $signature.SignerCertificate.Thumbprint) -ne $expected) {
        throw "Unexpected Authenticode signer for $(Split-Path -Leaf $Path)."
    }
    if ($TimestampRequired -and -not $signature.TimeStamperCertificate) {
        throw "A valid timestamp is required for $(Split-Path -Leaf $Path)."
    }
    if ($SignToolPath) {
        & $SignToolPath verify /pa /all $Path | Out-Null
        if ($LASTEXITCODE -ne 0) { throw "SignTool verification failed for $(Split-Path -Leaf $Path)." }
    }
}

function Assert-GraphLayerSignTool([string] $Path) {
    if (-not (Test-Path -LiteralPath $Path -PathType Leaf) -or [IO.Path]::GetFileName($Path) -ine 'signtool.exe') {
        throw 'Use an explicit Microsoft Windows SDK signtool.exe path.'
    }
    $signature = Get-AuthenticodeSignature -LiteralPath $Path
    if ($signature.Status -ne [Management.Automation.SignatureStatus]::Valid -or
        $signature.SignatureType -ne 'Authenticode' -or
        -not $signature.SignerCertificate -or
        $signature.SignerCertificate.Subject -notmatch '(^|,\s*)O=Microsoft Corporation(,|$)') {
        throw 'SignTool must have a valid embedded Microsoft Authenticode signature.'
    }
    if ((Get-Item -LiteralPath $Path).VersionInfo.OriginalFilename -ine 'SIGNTOOL.EXE') {
        throw 'The signed executable is not Microsoft SignTool.'
    }
    # Corporate Microsoft roots, not the Public Trust root used for third-party
    # signing customers. Source: https://www.microsoft.com/pkiops/docs/repository.htm
    $microsoftRoots = @(
        'DF545BF919A2439C36983B54CDFC903DFA4F37D3996D8D84B4C31EEC6F3C163E',
        '847DF6A78497943F27FC72EB93F9A637320A02B561D0A91B09E87A7807ED7C61'
    )
    $chain = [Security.Cryptography.X509Certificates.X509Chain]::new()
    $hasher = [Security.Cryptography.SHA256]::Create()
    try {
        # Authenticode above validates trust and timestamp semantics. This extra
        # chain build identifies the root, including timestamp-valid older SDKs.
        $chain.ChainPolicy.RevocationMode = [Security.Cryptography.X509Certificates.X509RevocationMode]::NoCheck
        $chain.ChainPolicy.VerificationFlags = [Security.Cryptography.X509Certificates.X509VerificationFlags]::IgnoreNotTimeValid
        if (-not $chain.Build($signature.SignerCertificate) -or $chain.ChainElements.Count -lt 2) {
            throw 'Unable to establish the Microsoft SignTool issuer chain.'
        }
        $root = $chain.ChainElements[$chain.ChainElements.Count - 1].Certificate
        $rootHash = [BitConverter]::ToString($hasher.ComputeHash($root.RawData)).Replace('-', '')
        if ($rootHash -notin $microsoftRoots) { throw 'SignTool issuer is not an approved Microsoft corporate root.' }
    } finally {
        $hasher.Dispose()
        $chain.Dispose()
    }
}

Export-ModuleMember -Function Test-GraphLayerPE, Get-GraphLayerSigningCertificate, Assert-GraphLayerSignedFile, Assert-GraphLayerSignTool
