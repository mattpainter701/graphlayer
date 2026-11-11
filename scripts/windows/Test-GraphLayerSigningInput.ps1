[CmdletBinding()]
param(
    [Parameter(Mandatory)] [string] $InputDirectory,
    [Parameter(Mandatory)] [ValidatePattern('^(0|[1-9][0-9]{0,4})\.(0|[1-9][0-9]{0,4})\.(0|[1-9][0-9]{0,4})$')] [string] $Version
)

$ErrorActionPreference = 'Stop'
foreach ($component in ($Version -split '\.')) {
    if ([int]::Parse($component, [Globalization.CultureInfo]::InvariantCulture) -gt 65535) {
        throw 'Each version component must be between 0 and 65535.'
    }
}
$scriptRoot = Split-Path -Parent $MyInvocation.MyCommand.Path
Import-Module (Join-Path $scriptRoot 'GraphLayer.WindowsPackage.psm1') -Force

$directory = Get-Item -LiteralPath $InputDirectory -Force
if (-not $directory.PSIsContainer) { throw 'InputDirectory must be a directory.' }
if ($directory.Attributes -band [IO.FileAttributes]::ReparsePoint) { throw 'InputDirectory must not be a reparse point.' }
$resolvedDirectory = (Resolve-Path -LiteralPath $InputDirectory).Path

$expectedNames = @('graphlayer-engine.exe', 'graphlayer-investigate.exe')
$items = @(Get-ChildItem -LiteralPath $resolvedDirectory -Force)
if ($items.Count -ne $expectedNames.Count) { throw 'Release input must contain exactly graphlayer-investigate.exe and graphlayer-engine.exe.' }
$actualNames = @($items | ForEach-Object Name | Sort-Object)
if (($actualNames -join '|') -cne (@($expectedNames | Sort-Object) -join '|')) {
    throw 'Release input must contain exactly graphlayer-investigate.exe and graphlayer-engine.exe.'
}
foreach ($item in $items) {
    if ($item.PSIsContainer -or ($item.Attributes -band [IO.FileAttributes]::ReparsePoint)) {
        throw "Release input entries must be regular files: $($item.Name)."
    }
}

$expectedVersion = $Version
$normalizedVersion = "$Version.0"
foreach ($name in $expectedNames) {
    $path = Join-Path $resolvedDirectory $name
    if ((Get-Item -LiteralPath $path -Force).Attributes -band [IO.FileAttributes]::ReparsePoint) { throw "Release input must not be a reparse point: $name." }
    if (-not (Test-GraphLayerPE -Path $path)) { throw "Release input is not a valid PE executable: $name." }
    $versionInfo = (Get-Item -LiteralPath $path).VersionInfo
    if ($versionInfo.ProductName -cne 'GraphLayer') { throw "Unexpected ProductName in $name; expected GraphLayer." }
    foreach ($field in @('ProductVersion', 'FileVersion')) {
        $actualVersion = [string]$versionInfo.$field
        if ($actualVersion -cne $expectedVersion -and $actualVersion -cne $normalizedVersion) {
            throw "Unexpected $field in $name; expected $expectedVersion or $normalizedVersion, got '$actualVersion'."
        }
    }
    if ([string]$versionInfo.OriginalFilename -cne $name) {
        throw "Unexpected OriginalFilename in $name; expected $name."
    }
}

Write-Output "Valid GraphLayer $Version release inputs: $resolvedDirectory"
