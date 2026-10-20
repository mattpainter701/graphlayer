param([string] $SignToolPath)

$ErrorActionPreference = 'Stop'
$repo = Split-Path -Parent $PSScriptRoot
$newPackage = Join-Path $repo 'scripts/windows/New-GraphLayerWindowsPackage.ps1'
$testPackage = Join-Path $repo 'scripts/windows/Test-GraphLayerWindowsPackage.ps1'
$temp = Join-Path ([IO.Path]::GetTempPath()) ("graphlayer-package-tests-" + [guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Path $temp | Out-Null

function Assert-Fails([scriptblock] $Action, [string] $Name) {
    try { & $Action; throw "Expected failure did not occur: $Name" }
    catch { if ($_.Exception.Message -eq "Expected failure did not occur: $Name") { throw } }
}

function Write-FakePE([string] $Path) {
    $bytes = [byte[]]::new(512)
    $bytes[0] = 0x4d; $bytes[1] = 0x5a
    $bytes[0x3c] = 0x80
    $bytes[0x80] = 0x50; $bytes[0x81] = 0x45
    [IO.File]::WriteAllBytes($Path, $bytes)
}

function Write-Manifest([string] $Directory, [string] $Kind, [string] $Signer) {
    $names = @('graphlayer-engine.exe', 'graphlayer-investigate.exe')
    $entries = @($names | ForEach-Object { [ordered]@{ name = $_; sha256 = (Get-FileHash (Join-Path $Directory $_) -Algorithm SHA256).Hash.ToLowerInvariant() } })
    $mode = if ($Kind -eq 'Signed') { 'PreSigned' } else { $null }
    $manifest = [ordered]@{ schema='graphlayer-windows-package-v1'; package_kind=$Kind; signing_mode=$mode; commit=('a' * 40); signer_thumbprint=$Signer; files=$entries }
    [IO.File]::WriteAllText((Join-Path $Directory 'manifest.json'), ($manifest | ConvertTo-Json -Depth 5), [Text.UTF8Encoding]::new($false))
    $sums = ($entries | ForEach-Object { "$($_.sha256)  $($_.name)" }) -join "`n"
    [IO.File]::WriteAllText((Join-Path $Directory 'SHA256SUMS'), $sums + "`n", [Text.UTF8Encoding]::new($false))
}

function Resolve-VerifiedSignTool([string] $Candidate) {
    if (-not $Candidate) {
        $sdkRoot = 'C:\Program Files (x86)\Windows Kits\10\bin'
        if (-not (Test-Path -LiteralPath $sdkRoot -PathType Container)) { throw 'Pass -SignToolPath or install the Windows SDK.' }
        $Candidate = Get-ChildItem -LiteralPath $sdkRoot -Filter signtool.exe -Recurse -File -ErrorAction SilentlyContinue |
            Where-Object { $_.FullName -match '\\x64\\signtool\.exe$' } | Select-Object -First 1 -ExpandProperty FullName
    }
    if (-not $Candidate -or [IO.Path]::GetFileName($Candidate) -ine 'signtool.exe' -or -not (Test-Path -LiteralPath $Candidate -PathType Leaf)) {
        throw 'SignToolPath must identify an existing signtool.exe.'
    }
    $signature = Get-AuthenticodeSignature -LiteralPath $Candidate
    if ($signature.Status -ne 'Valid' -or $signature.SignatureType -ne 'Authenticode' -or -not $signature.TimeStamperCertificate -or
        -not $signature.SignerCertificate.Subject.StartsWith('CN=Microsoft Corporation, O=Microsoft Corporation,', [StringComparison]::OrdinalIgnoreCase)) {
        throw 'SignToolPath must be a valid, timestamped Microsoft Authenticode-signed executable.'
    }
    return [IO.Path]::GetFullPath($Candidate)
}

function Remove-TestDirectory([string] $CreatedPath) {
    $expected = [IO.Path]::GetFullPath($CreatedPath)
    if (-not (Test-Path -LiteralPath $expected -PathType Container)) { return }
    $resolved = [IO.Path]::GetFullPath((Resolve-Path -LiteralPath $expected).Path)
    $item = Get-Item -LiteralPath $resolved -Force
    if ($resolved -ne $expected -or ($item.Attributes -band [IO.FileAttributes]::ReparsePoint)) {
        throw "Refusing recursive cleanup because the test directory no longer resolves to its created path: $expected"
    }
    Remove-Item -LiteralPath $expected -Recurse -Force
}

try {
    $signTool = Resolve-VerifiedSignTool $SignToolPath
    $packageInput = Join-Path $temp 'input'
    New-Item -ItemType Directory -Path $packageInput | Out-Null
    Write-FakePE (Join-Path $packageInput 'graphlayer-engine.exe')
    Write-FakePE (Join-Path $packageInput 'graphlayer-investigate.exe')
    $development = Join-Path $temp 'development'
    & $newPackage -InputDirectory $packageInput -OutputDirectory $development -PackageKind Development -SourceRevision ('a' * 40) | Out-Null
    & $testPackage -PackageDirectory $development | Out-Null
    Assert-Fails { & $testPackage -PackageDirectory $development -RequireSigned -ExpectedSignerThumbprint ('0' * 40) } 'unsigned package accepted as signed'
    $extra = Join-Path $development 'extra.exe'
    Copy-Item (Join-Path $packageInput 'graphlayer-engine.exe') $extra
    Assert-Fails { & $testPackage -PackageDirectory $development } 'extra executable accepted'
    Remove-Item -LiteralPath $extra
    Write-Manifest $development 'Signed' ('0' * 40)
    Assert-Fails { & $testPackage -PackageDirectory $development -RequireSigned -ExpectedSignerThumbprint ('0' * 40) -SignToolPath $signTool } 'unsigned executables accepted with a signed manifest'
    Assert-Fails { & $testPackage -PackageDirectory $development -RequireSigned -ExpectedSignerThumbprint ('0' * 39) -SignToolPath $signTool } 'short signer thumbprint accepted'

    $invalidTimeOutput = Join-Path $temp 'invalid-time'
    Assert-Fails { & $newPackage -InputDirectory $packageInput -OutputDirectory $invalidTimeOutput -PackageKind Signed -CertificateThumbprint ('1' * 40) -SignToolPath $signTool -TimestampServer 'http://timestamp.example.invalid' -SourceRevision ('a' * 40) } 'HTTP timestamp URL accepted'
    if (Test-Path -LiteralPath $invalidTimeOutput) { throw 'Invalid timestamp preflight created an output directory.' }
    $shortThumbOutput = Join-Path $temp 'short-thumb'
    Assert-Fails { & $newPackage -InputDirectory $packageInput -OutputDirectory $shortThumbOutput -PackageKind Signed -CertificateThumbprint ('1' * 39) -SignToolPath $signTool -TimestampServer 'https://timestamp.example.invalid' -SourceRevision ('a' * 40) } 'short certificate thumbprint accepted'
    if (Test-Path -LiteralPath $shortThumbOutput) { throw 'Invalid thumbprint preflight created an output directory.' }

    $signedFixture = Join-Path $temp 'signed-fixture'
    New-Item -ItemType Directory -Path $signedFixture | Out-Null
    $signature = Get-AuthenticodeSignature -LiteralPath $signTool
    $thumb = $signature.SignerCertificate.Thumbprint.ToUpperInvariant()
    $unsignedSignedLabel = Join-Path $temp 'unsigned-signed-label'
    New-Item -ItemType Directory -Path $unsignedSignedLabel | Out-Null
    Copy-Item (Join-Path $packageInput 'graphlayer-engine.exe') (Join-Path $unsignedSignedLabel 'graphlayer-engine.exe')
    Copy-Item (Join-Path $packageInput 'graphlayer-investigate.exe') (Join-Path $unsignedSignedLabel 'graphlayer-investigate.exe')
    Write-Manifest $unsignedSignedLabel 'Signed' $thumb
    Assert-Fails { & $testPackage -PackageDirectory $unsignedSignedLabel -RequireSigned -ExpectedSignerThumbprint $thumb -SignToolPath $signTool } 'unsigned PE files accepted with the expected signed manifest'

    Copy-Item $signTool (Join-Path $signedFixture 'graphlayer-engine.exe')
    Copy-Item $signTool (Join-Path $signedFixture 'graphlayer-investigate.exe')
    Write-Manifest $signedFixture 'Signed' $thumb
    & $testPackage -PackageDirectory $signedFixture -RequireSigned -ExpectedSignerThumbprint $thumb -SignToolPath $signTool | Out-Null
    Assert-Fails { & $testPackage -PackageDirectory $signedFixture -RequireSigned -ExpectedSignerThumbprint ('0' * 40) -SignToolPath $signTool } 'wrong signer accepted'

    $preSigned = Join-Path $temp 'pre-signed-package'
    & $newPackage -InputDirectory $signedFixture -OutputDirectory $preSigned -PackageKind Signed -SigningMode PreSigned -CertificateThumbprint $thumb -SignToolPath $signTool -SourceRevision ('b' * 40) | Out-Null
    & $testPackage -PackageDirectory $preSigned -RequireSigned -ExpectedSignerThumbprint $thumb -SignToolPath $signTool | Out-Null

    $extraDll = Join-Path $temp 'extra-dll'
    Copy-Item $signedFixture $extraDll -Recurse
    Copy-Item $signTool (Join-Path $extraDll 'extra.dll')
    Assert-Fails { & $testPackage -PackageDirectory $extraDll -RequireSigned -ExpectedSignerThumbprint $thumb -SignToolPath $signTool } 'extra DLL accepted'

    $nested = Join-Path $temp 'nested-executable'
    Copy-Item $signedFixture $nested -Recurse
    $nestedDirectory = Join-Path $nested 'nested'
    New-Item -ItemType Directory -Path $nestedDirectory | Out-Null
    Copy-Item $signTool (Join-Path $nestedDirectory 'extra.exe')
    Assert-Fails { & $testPackage -PackageDirectory $nested -RequireSigned -ExpectedSignerThumbprint $thumb -SignToolPath $signTool } 'nested executable accepted'

    $tampered = Join-Path $temp 'tampered'
    Copy-Item $signedFixture $tampered -Recurse
    [IO.File]::AppendAllText((Join-Path $tampered 'graphlayer-engine.exe'), 'tamper')
    Assert-Fails { & $testPackage -PackageDirectory $tampered -RequireSigned -ExpectedSignerThumbprint $thumb -SignToolPath $signTool } 'tampered executable accepted'

    $missing = Join-Path $temp 'missing-worker'
    Copy-Item $signedFixture $missing -Recurse
    Remove-Item (Join-Path $missing 'graphlayer-engine.exe')
    Assert-Fails { & $testPackage -PackageDirectory $missing -RequireSigned -ExpectedSignerThumbprint $thumb -SignToolPath $signTool } 'missing worker accepted'

    Write-Output 'Windows package contracts passed.'
} finally {
    Remove-TestDirectory $temp
}
