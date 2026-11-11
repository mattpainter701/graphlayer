param(
    [string] $RcPath,
    [string] $CvtresPath,
    [string] $RawObjectPath,
    [string] $NormalizedObjectPath
)

$ErrorActionPreference = 'Stop'
$repo = Split-Path -Parent $PSScriptRoot
$modulePath = Join-Path $repo 'scripts/windows/GraphLayerCoff.psm1'
Import-Module -Name $modulePath -Force -ErrorAction Stop
$fixtureMode = -not [string]::IsNullOrWhiteSpace($RawObjectPath) -or -not [string]::IsNullOrWhiteSpace($NormalizedObjectPath)
if ($fixtureMode -and ([string]::IsNullOrWhiteSpace($RawObjectPath) -or [string]::IsNullOrWhiteSpace($NormalizedObjectPath))) {
    throw 'Pass both -RawObjectPath and -NormalizedObjectPath for metadata-only COFF fixture mode.'
}
$temp = Join-Path ([IO.Path]::GetTempPath()) ('graphlayer-coff-resource-' + [guid]::NewGuid().ToString('N'))
$null = New-Item -ItemType Directory -Path $temp

function Assert-Fails([scriptblock] $Action, [string] $Name) {
    try { & $Action; throw "Expected failure did not occur: $Name" }
    catch { if ($_.Exception.Message -eq "Expected failure did not occur: $Name") { throw } }
}

function Copy-Metadata([object] $Metadata) {
    return ($Metadata | ConvertTo-Json -Depth 30 | ConvertFrom-Json -AsHashtable)
}

function Resolve-TestTool([string] $Candidate, [string] $Name, [string] $Root) {
    if (-not $Candidate) {
        $Candidate = Get-ChildItem -LiteralPath $Root -Filter $Name -Recurse -File -ErrorAction SilentlyContinue |
            Select-Object -First 1 -ExpandProperty FullName
    }
    if (-not $Candidate -or -not (Test-Path -LiteralPath $Candidate -PathType Leaf)) {
        throw "The Windows SDK/Visual Studio $Name is required for the COFF contract test."
    }
    return [IO.Path]::GetFullPath($Candidate)
}

function Invoke-TestTool([string] $Path, [string[]] $Arguments, [string] $LogPath, [string] $Name) {
    & $Path @Arguments *> $LogPath
    if ($LASTEXITCODE -ne 0) { throw "$Name failed in the Windows COFF contract test (exit code $LASTEXITCODE)." }
}

function Assert-MarkerFailure([object] $Metadata, [string] $Name) {
    Assert-Fails { Assert-GraphLayerCoffInput $Metadata | Out-Null } $Name
}

function Remove-TestDirectory([string] $CreatedPath) {
    if (-not (Test-Path -LiteralPath $CreatedPath -PathType Container)) { return }
    $expected = [IO.Path]::GetFullPath($CreatedPath)
    $resolved = [IO.Path]::GetFullPath((Resolve-Path -LiteralPath $expected).Path)
    $item = Get-Item -LiteralPath $resolved -Force
    if ($resolved -ne $expected -or ($item.Attributes -band [IO.FileAttributes]::ReparsePoint)) {
        throw 'Refusing COFF test cleanup because the temporary directory changed identity.'
    }
    Remove-Item -LiteralPath $expected -Recurse -Force
}

try {
    $sdkRoot = 'C:\Program Files (x86)\Windows Kits\10\bin'
    $vsRoot = 'C:\Program Files\Microsoft Visual Studio'
    if ($fixtureMode) {
        $rawObject = [IO.Path]::GetFullPath($RawObjectPath)
        $normalizedObject = [IO.Path]::GetFullPath($NormalizedObjectPath)
    } else {
        if ([string]::IsNullOrWhiteSpace($RcPath) -and -not (Test-Path -LiteralPath $sdkRoot -PathType Container)) {
            throw 'The Windows SDK is required for the COFF contract test.'
        }
        if ([string]::IsNullOrWhiteSpace($CvtresPath) -and -not (Test-Path -LiteralPath $vsRoot -PathType Container)) {
            throw 'Visual Studio Build Tools are required for the COFF contract test.'
        }
        $rc = Resolve-TestTool $RcPath 'rc.exe' $sdkRoot
        $cvtres = Resolve-TestTool $CvtresPath 'cvtres.exe' $vsRoot
        $objcopy = Resolve-GraphLayerPinnedObjcopy

        # Use Microsoft's resource compiler to create a real amd64 COFF fixture.
        # No handwritten COFF object is used by this contract test.
        $resourceScript = Join-Path $temp 'fixture.rc'
        $resource = Join-Path $temp 'fixture.res'
        $rawObject = Join-Path $temp 'fixture.raw.syso'
        $normalizedObject = Join-Path $temp 'fixture.normalized.syso'
        $rcLog = Join-Path $temp 'rc.log'
        $cvtresLog = Join-Path $temp 'cvtres.log'
        $resourceText = @'
LANGUAGE 9, 1
1 VERSIONINFO
 FILEVERSION 1,2,3,0
 PRODUCTVERSION 1,2,3,0
 FILEFLAGSMASK 0x3fL
 FILEFLAGS 0x0L
 FILEOS 0x00040004L
 FILETYPE 0x00000001L
 FILESUBTYPE 0x00000000L
BEGIN
  BLOCK "StringFileInfo"
  BEGIN
    BLOCK "040904B0"
    BEGIN
      VALUE "FileDescription", "COFF contract fixture\0"
      VALUE "FileVersion", "1.2.3\0"
      VALUE "InternalName", "coff-fixture\0"
      VALUE "OriginalFilename", "coff-fixture.exe\0"
      VALUE "ProductName", "GraphLayer\0"
      VALUE "ProductVersion", "1.2.3\0"
    END
  END
  BLOCK "VarFileInfo"
  BEGIN
    VALUE "Translation", 0x0409, 1200
  END
END
'@
        [IO.File]::WriteAllText($resourceScript, $resourceText, [Text.UTF8Encoding]::new($false))
        Invoke-TestTool $rc @('/nologo', "/fo$resource", $resourceScript) $rcLog 'rc.exe'
        Invoke-TestTool $cvtres @('/nologo', '/MACHINE:X64', "/OUT:$rawObject", $resource) $cvtresLog 'cvtres.exe'
    }

    $before = Get-GraphLayerCoffMetadata $rawObject
    $marker = Assert-GraphLayerCoffInput $before
    if ($before.Machine -ne 0x8664 -or $marker.Name -cne '@comp.id') { throw 'CVTRES did not produce the expected amd64 marker fixture.' }
    if (-not $fixtureMode) {
        $null = Normalize-GraphLayerGoResource -InputPath $rawObject -OutputPath $normalizedObject -ObjcopyPath $objcopy
    }
    $after = Get-GraphLayerCoffMetadata $normalizedObject
    Assert-GraphLayerCoffNormalized $before $after

    $missing = Copy-Metadata $before
    $missing.Symbols = @($missing.Symbols | Where-Object { $_.Name -cne '@comp.id' })
    Assert-MarkerFailure $missing 'missing marker'

    $unknown = Copy-Metadata $before
    ($unknown.Symbols | Where-Object { $_.Name -ceq '@comp.id' } | Select-Object -First 1).Name = '@unknown'
    Assert-MarkerFailure $unknown 'unknown marker name'

    $duplicate = Copy-Metadata $before
    $duplicate.Symbols += ,(Copy-Metadata $marker)
    Assert-MarkerFailure $duplicate 'duplicate marker'

    $changedClass = Copy-Metadata $before
    ($changedClass.Symbols | Where-Object { $_.Name -ceq '@comp.id' } | Select-Object -First 1).StorageClass = 2
    Assert-MarkerFailure $changedClass 'marker storage class changed'

    $changedSection = Copy-Metadata $before
    ($changedSection.Symbols | Where-Object { $_.Name -ceq '@comp.id' } | Select-Object -First 1).Section = 1
    Assert-MarkerFailure $changedSection 'marker section changed'

    $changedType = Copy-Metadata $before
    ($changedType.Symbols | Where-Object { $_.Name -ceq '@comp.id' } | Select-Object -First 1).Type = 1
    Assert-MarkerFailure $changedType 'marker type changed'

    $changedAux = Copy-Metadata $before
    ($changedAux.Symbols | Where-Object { $_.Name -ceq '@comp.id' } | Select-Object -First 1).AuxCount = 1
    Assert-MarkerFailure $changedAux 'marker auxiliary record changed'

    $referenced = Copy-Metadata $before
    $referenced.Sections[0].Relocations += ,([ordered]@{ Address = 0; TargetIndex = $marker.Index; Type = 1 })
    Assert-MarkerFailure $referenced 'marker referenced by relocation'

    $wrongMachine = Copy-Metadata $before
    $wrongMachine.Machine = 0x14c
    Assert-MarkerFailure $wrongMachine 'wrong machine'

    $lineTable = Copy-Metadata $before
    $lineTable.Sections[0].LineCount = 1
    Assert-MarkerFailure $lineTable 'unsupported line-number table'

    $corruptOutput = Copy-Metadata $after
    $corruptOutput.Sections[0].RawHash = '00'
    Assert-Fails { Assert-GraphLayerCoffNormalized $before $corruptOutput } 'changed section bytes'

    $changedTimestamp = Copy-Metadata $after
    $changedTimestamp.Timestamp = $changedTimestamp.Timestamp + 1
    Assert-Fails { Assert-GraphLayerCoffNormalized $before $changedTimestamp } 'changed COFF timestamp'

    $changedSymbolPointer = Copy-Metadata $after
    $changedSymbolPointer.SymbolPointer = $changedSymbolPointer.SymbolPointer + 1
    Assert-Fails { Assert-GraphLayerCoffNormalized $before $changedSymbolPointer } 'changed COFF symbol-table pointer'

    $changedStringContent = Copy-Metadata $after
    $changedStringContent.StringPayloadHash = '00'
    Assert-Fails { Assert-GraphLayerCoffNormalized $before $changedStringContent } 'changed COFF string-table content'

    $changedAux = Copy-Metadata $after
    $sectionSymbol = $changedAux.Symbols | Where-Object { $_.Name -ceq '.rsrc$01' } | Select-Object -First 1
    if (-not $sectionSymbol -or $sectionSymbol.AuxCount -ne 1) { throw 'Expected section-definition auxiliary fixture was not found.' }
    $auxBytes = [Convert]::FromHexString($sectionSymbol.AuxHex)
    $auxBytes[14] = 5
    $sectionSymbol.AuxHex = [Convert]::ToHexString($auxBytes)
    Assert-Fails { Assert-GraphLayerCoffNormalized $before $changedAux } 'changed COMDAT auxiliary metadata'

    $beforeSelfNumber = Copy-Metadata $before
    $beforeSelfSymbol = $beforeSelfNumber.Symbols | Where-Object { $_.Name -ceq '.rsrc$01' } | Select-Object -First 1
    if (-not $beforeSelfSymbol -or $beforeSelfSymbol.AuxCount -ne 1) { throw 'Expected section-definition auxiliary fixture was not found.' }
    $beforeSelfBytes = [Convert]::FromHexString($beforeSelfSymbol.AuxHex)
    $sectionNumber = [uint16]$beforeSelfSymbol.Section
    $beforeSelfBytes[12] = [byte]($sectionNumber -band 0xff)
    $beforeSelfBytes[13] = [byte](($sectionNumber -shr 8) -band 0xff)
    $beforeSelfSymbol.AuxHex = [Convert]::ToHexString($beforeSelfBytes)
    $reversedAux = Copy-Metadata $after
    $reversedSymbol = $reversedAux.Symbols | Where-Object { $_.Name -ceq '.rsrc$01' } | Select-Object -First 1
    $reversedBytes = [Convert]::FromHexString($reversedSymbol.AuxHex)
    $reversedBytes[12] = 0
    $reversedBytes[13] = 0
    $reversedSymbol.AuxHex = [Convert]::ToHexString($reversedBytes)
    Assert-Fails { Assert-GraphLayerCoffNormalized $beforeSelfNumber $reversedAux } 'reversed section Number normalization'

    $missingSysroot = Join-Path $temp 'missing-rust-sysroot'
    Assert-Fails { Resolve-GraphLayerObjcopyAtSysroot $missingSysroot 'x86_64-pc-windows-msvc' } 'missing pinned objcopy component'
    Write-Output 'Windows CVTRES COFF normalization contracts passed.'
} finally {
    Remove-TestDirectory $temp
}
