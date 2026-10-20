[CmdletBinding()]
param(
    [Parameter(Mandatory)] [string] $Version,
    [Parameter(Mandatory)] [string] $RcPath,
    [Parameter(Mandatory)] [string] $CvtresPath,
    [Parameter(Mandatory)] [string] $OutputDirectory
)

$ErrorActionPreference = 'Stop'

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
    & $rc /nologo "/fo$appRes" $appRc
    if ($LASTEXITCODE -ne 0) { throw "rc.exe failed to compile the application resource with exit code $LASTEXITCODE." }
    & $rc /nologo "/fo$workerRes" $workerRc
    if ($LASTEXITCODE -ne 0) { throw "rc.exe failed to compile the worker resource with exit code $LASTEXITCODE." }

    $appSyso = Join-Path $output 'graphlayer-investigate.syso'
    & $cvtres /nologo /MACHINE:X64 "/OUT:$appSyso" $appRes
    if ($LASTEXITCODE -ne 0) { throw "cvtres.exe failed to create the Go resource object with exit code $LASTEXITCODE." }
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
