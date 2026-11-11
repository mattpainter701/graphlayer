param(
    [string] $ReleaseInputDirectory,
    [string] $Version = '1.2.3'
)

$ErrorActionPreference = 'Stop'
$repo = Split-Path -Parent $PSScriptRoot
$validator = Join-Path $repo 'scripts/windows/Test-GraphLayerSigningInput.ps1'
$temp = Join-Path ([IO.Path]::GetTempPath()) ("graphlayer-signing-input-tests-" + [guid]::NewGuid().ToString('N'))
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

function New-FakeInput([string] $Directory) {
    New-Item -ItemType Directory -Path $Directory | Out-Null
    Write-FakePE (Join-Path $Directory 'graphlayer-investigate.exe')
    Write-FakePE (Join-Path $Directory 'graphlayer-engine.exe')
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
    $validShape = Join-Path $temp 'valid-shape'
    New-FakeInput $validShape
    Assert-Fails { & $validator -InputDirectory $validShape -Version '1.2' } 'non X.Y.Z version accepted'
    Assert-Fails { & $validator -InputDirectory $validShape -Version '01.2.3' } 'leading-zero version accepted'
    Assert-Fails { & $validator -InputDirectory $validShape -Version '1.2.3-preview' } 'prerelease version accepted'
    Assert-Fails { & $validator -InputDirectory $validShape -Version '65536.0.0' } 'out-of-range version accepted'
    Assert-Fails { & $validator -InputDirectory $validShape -Version '1.2.3' } 'PE without release metadata accepted'

    $extraFile = Join-Path $temp 'extra-file'
    New-FakeInput $extraFile
    [IO.File]::WriteAllText((Join-Path $extraFile 'extra.txt'), 'unexpected')
    Assert-Fails { & $validator -InputDirectory $extraFile -Version '1.2.3' } 'extra file accepted'

    $extraDirectory = Join-Path $temp 'extra-directory'
    New-FakeInput $extraDirectory
    New-Item -ItemType Directory -Path (Join-Path $extraDirectory 'nested') | Out-Null
    Assert-Fails { & $validator -InputDirectory $extraDirectory -Version '1.2.3' } 'extra directory accepted'

    $nonPe = Join-Path $temp 'non-pe'
    New-Item -ItemType Directory -Path $nonPe | Out-Null
    [IO.File]::WriteAllText((Join-Path $nonPe 'graphlayer-investigate.exe'), 'not a PE')
    Write-FakePE (Join-Path $nonPe 'graphlayer-engine.exe')
    Assert-Fails { & $validator -InputDirectory $nonPe -Version '1.2.3' } 'non-PE release input accepted'

    if ($ReleaseInputDirectory) {
        & $validator -InputDirectory $ReleaseInputDirectory -Version $Version | Out-Null
    }
    Write-Output 'Windows signing input contracts passed.'
} finally {
    Remove-TestDirectory $temp
}
