[CmdletBinding()]
param(
    [Parameter(Mandatory)] [string] $Version,
    [Parameter(Mandatory)] [string] $RcPath,
    [Parameter(Mandatory)] [string] $CvtresPath,
    [Parameter(Mandatory)] [string] $OutputDirectory
)

$ErrorActionPreference = 'Stop'
$coffModule = Join-Path $PSScriptRoot 'GraphLayerCoff.psm1'
Import-Module -Name $coffModule -Force -ErrorAction Stop

function ConvertTo-VersionParts([string] $Value) {
    if ($Value -notmatch '^(0|[1-9][0-9]{0,4})\.(0|[1-9][0-9]{0,4})\.(0|[1-9][0-9]{0,4})$') {
        throw 'Version must use strict X.Y.Z notation with decimal components from 0 through 65535.'
    }
    $parts = @(
        [int]::Parse($Matches[1], [Globalization.CultureInfo]::InvariantCulture),
        [int]::Parse($Matches[2], [Globalization.CultureInfo]::InvariantCulture),
        [int]::Parse($Matches[3], [Globalization.CultureInfo]::InvariantCulture)
    )
    if (@($parts | Where-Object { $_ -gt 65535 }).Count -gt 0) {
        throw 'Each version component must be between 0 and 65535.'
    }
    return ,$parts
}

function Resolve-VerifiedMicrosoftTool([string] $Path, [string] $ExpectedName) {
    $resolved = [IO.Path]::GetFullPath($Path)
    if ([IO.Path]::GetFileName($resolved) -ine $ExpectedName -or -not (Test-Path -LiteralPath $resolved -PathType Leaf)) {
        throw "$ExpectedName path must identify an existing $ExpectedName."
    }
    $signature = Get-AuthenticodeSignature -LiteralPath $resolved
    if ($signature.Status -ne [Management.Automation.SignatureStatus]::Valid -or
        $signature.SignatureType -ne 'Authenticode' -or
        -not $signature.SignerCertificate -or
        $signature.SignerCertificate.Subject -notmatch '(^|,\s*)O=Microsoft Corporation(,|$)' -or
        (Get-Item -LiteralPath $resolved).VersionInfo.OriginalFilename -ine $ExpectedName) {
        throw "$ExpectedName must have a valid embedded Microsoft Authenticode signature and matching original filename."
    }
    return $resolved
}

function Invoke-MicrosoftTool([string] $Path, [string[]] $Arguments, [string] $ToolName) {
    # Tool output can contain local paths. Suppress it and report only a bounded,
    # stable summary so CI logs do not expose host-specific command diagnostics.
    & $Path @Arguments 2>&1 | Out-Null
    $exitCode = $LASTEXITCODE
    if ($exitCode -ne 0) {
        throw "$ToolName failed with exit code $exitCode; compiler diagnostics were suppressed."
    }
}

function New-VersionResource([string] $ResourcePath, [string] $Description, [string] $InternalName, [string] $OriginalName, [string] $VersionText, [int[]] $Parts) {
    $fileVersion = '{0},{1},{2},0' -f $Parts[0], $Parts[1], $Parts[2]
$content = @"
LANGUAGE 9, 1

1 VERSIONINFO
 FILEVERSION $fileVersion
 PRODUCTVERSION $fileVersion
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
      VALUE "CompanyName", "GraphLayer\0"
      VALUE "FileDescription", "$Description\0"
      VALUE "FileVersion", "$VersionText\0"
      VALUE "InternalName", "$InternalName\0"
      VALUE "OriginalFilename", "$OriginalName\0"
      VALUE "ProductName", "GraphLayer\0"
      VALUE "ProductVersion", "$VersionText\0"
    END
  END
  BLOCK "VarFileInfo"
  BEGIN
    VALUE "Translation", 0x0409, 1200
  END
END
"@
    [IO.File]::WriteAllText($ResourcePath, $content.TrimStart(), [Text.UTF8Encoding]::new($false))
}

$parts = ConvertTo-VersionParts $Version
$rc = Resolve-VerifiedMicrosoftTool -Path $RcPath -ExpectedName 'rc.exe'
$cvtres = Resolve-VerifiedMicrosoftTool -Path $CvtresPath -ExpectedName 'cvtres.exe'
$objcopy = Resolve-GraphLayerPinnedObjcopy
$output = [IO.Path]::GetFullPath($OutputDirectory)
if (Test-Path -LiteralPath $output) { throw "Output directory already exists: $output" }
$parent = Split-Path -Parent $output
if (-not (Test-Path -LiteralPath $parent -PathType Container)) { throw "Output parent directory does not exist: $parent" }

$versionText = '{0}.{1}.{2}' -f $parts[0], $parts[1], $parts[2]
$null = New-Item -ItemType Directory -Path $output
$createdOutput = $output
try {
    $appRc = Join-Path $output 'graphlayer-investigate.rc'
    $workerRc = Join-Path $output 'graphlayer-engine.rc'
    New-VersionResource $appRc 'GraphLayer Investigation Application' 'graphlayer-investigate' 'graphlayer-investigate.exe' $versionText $parts
    New-VersionResource $workerRc 'GraphLayer Rust Analysis Engine' 'graphlayer-engine' 'graphlayer-engine.exe' $versionText $parts

    $appRes = Join-Path $output 'graphlayer-investigate-resource.res'
    $workerRes = Join-Path $output 'graphlayer-engine.res'
    Invoke-MicrosoftTool $rc @('/nologo', "/fo$appRes", $appRc) 'rc.exe application resource compilation'
    Invoke-MicrosoftTool $rc @('/nologo', "/fo$workerRes", $workerRc) 'rc.exe worker resource compilation'
    foreach ($resource in @($appRes, $workerRes)) {
        if (-not (Test-Path -LiteralPath $resource -PathType Leaf)) {
            throw 'rc.exe did not create the expected compiled resource file.'
        }
    }

    $appRawSyso = Join-Path $output 'graphlayer-investigate-raw.syso'
    $appCandidateSyso = Join-Path $output 'graphlayer-investigate-normalized.syso'
    $appSyso = Join-Path $output 'graphlayer-investigate.syso'
    Invoke-MicrosoftTool $cvtres @('/nologo', '/MACHINE:X64', "/OUT:$appRawSyso", $appRes) 'cvtres.exe Go application resource conversion'
    if (-not (Test-Path -LiteralPath $appRawSyso -PathType Leaf)) {
        throw 'cvtres.exe did not create the expected Go application resource object.'
    }
    $null = Normalize-GraphLayerGoResource -InputPath $appRawSyso -OutputPath $appCandidateSyso -ObjcopyPath $objcopy
    Move-Item -LiteralPath $appCandidateSyso -Destination $appSyso
    Remove-Item -LiteralPath $appRawSyso -Force
    Remove-Item -LiteralPath $appRes -Force
    Write-Output $output
} catch {
    if (Test-Path -LiteralPath $createdOutput -PathType Container) {
        $resolved = [IO.Path]::GetFullPath((Resolve-Path -LiteralPath $createdOutput).Path)
        $item = Get-Item -LiteralPath $resolved -Force
        if ($resolved -eq $createdOutput -and -not ($item.Attributes -band [IO.FileAttributes]::ReparsePoint)) {
            Remove-Item -LiteralPath $createdOutput -Recurse -Force -ErrorAction SilentlyContinue
        }
    }
    throw
}
