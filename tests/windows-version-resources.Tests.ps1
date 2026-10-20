param(
    [string] $RcPath,
    [string] $CvtresPath
)

$ErrorActionPreference = 'Stop'
$repo = Split-Path -Parent $PSScriptRoot
$newResources = Join-Path $repo 'scripts/windows/New-GraphLayerVersionResources.ps1'
$temp = Join-Path ([IO.Path]::GetTempPath()) ('graphlayer-version-resources-' + [guid]::NewGuid().ToString('N'))
$null = New-Item -ItemType Directory -Path $temp

function Assert-Fails([scriptblock] $Action, [string] $Name) {
    try { & $Action; throw "Expected failure did not occur: $Name" }
    catch { if ($_.Exception.Message -eq "Expected failure did not occur: $Name") { throw } }
}

function Remove-TestDirectory([string] $CreatedPath) {
    $expected = [IO.Path]::GetFullPath($CreatedPath)
    if (-not (Test-Path -LiteralPath $expected -PathType Container)) { return }
    $resolved = [IO.Path]::GetFullPath((Resolve-Path -LiteralPath $expected).Path)
    $item = Get-Item -LiteralPath $resolved -Force
    if ($resolved -ne $expected -or ($item.Attributes -band [IO.FileAttributes]::ReparsePoint)) {
        throw "Refusing cleanup because the test directory no longer resolves to its created path: $expected"
    }
    Remove-Item -LiteralPath $expected -Recurse -Force
}

function Resolve-MicrosoftTool([string] $Candidate, [string] $Name, [string] $Root) {
    if (-not $Candidate) {
        $Candidate = Get-ChildItem -LiteralPath $Root -Filter $Name -Recurse -File -ErrorAction SilentlyContinue |
            Select-Object -First 1 -ExpandProperty FullName
    }
    if (-not $Candidate) { throw "Pass -$($Name.Replace('.exe', 'Path')) or install the Microsoft SDK/Visual Studio Build Tools." }
    return [IO.Path]::GetFullPath($Candidate)
}

function Assert-VersionInfo([string] $Path, [string] $Version, [string] $Description, [string] $InternalName, [string] $OriginalName) {
    $info = [Diagnostics.FileVersionInfo]::GetVersionInfo($Path)
    if ($info.ProductName -ne 'GraphLayer' -or $info.ProductVersion -ne $Version -or
        $info.FileVersion -ne $Version -or $info.FileDescription -ne $Description -or
        $info.InternalName -ne $InternalName -or $info.OriginalFilename -ne $OriginalName) {
        throw "PE version resource mismatch in $(Split-Path -Leaf $Path)."
    }
}

try {
    foreach ($badVersion in @('1.2', '1.2.3.4', '-1.2.3', '01.2.3', '65536.0.0', '1.2.3-rc1', '1.2.x')) {
        $candidate = Join-Path $temp ('invalid-' + [guid]::NewGuid().ToString('N'))
        Assert-Fails { & $newResources -Version $badVersion -RcPath 'rc.exe' -CvtresPath 'cvtres.exe' -OutputDirectory $candidate } "invalid version $badVersion accepted"
        if (Test-Path -LiteralPath $candidate) { throw "Invalid version created output: $badVersion" }
    }

    $sdkRoot = 'C:\Program Files (x86)\Windows Kits\10\bin'
    if (-not (Test-Path -LiteralPath $sdkRoot -PathType Container)) { throw 'The Windows SDK is required for resource compilation tests.' }
    $rc = Resolve-MicrosoftTool $RcPath 'rc.exe' $sdkRoot
    if (-not $CvtresPath) {
        $vsRoot = 'C:\Program Files\Microsoft Visual Studio'
        if (-not (Test-Path -LiteralPath $vsRoot -PathType Container)) { throw 'Visual Studio Build Tools are required for cvtres.exe.' }
        $CvtresPath = Get-ChildItem -LiteralPath $vsRoot -Filter cvtres.exe -Recurse -File -ErrorAction SilentlyContinue |
            Where-Object { $_.FullName -match '\\Hostx64\\x64\\cvtres\.exe$' } | Select-Object -First 1 -ExpandProperty FullName
    }
    $cvtres = Resolve-MicrosoftTool $CvtresPath 'cvtres.exe' 'C:\Program Files\Microsoft Visual Studio'
    $go = Get-Command go.exe -ErrorAction SilentlyContinue
    if (-not $go) { throw 'Go is required to validate the generated PE version resources.' }

    $output = Join-Path $temp 'resources'
    & $newResources -Version '12.34.56' -RcPath $rc -CvtresPath $cvtres -OutputDirectory $output | Out-Null
    foreach ($name in @('graphlayer-investigate.rc', 'graphlayer-engine.rc', 'graphlayer-engine.res', 'graphlayer-investigate.syso')) {
        if (-not (Test-Path -LiteralPath (Join-Path $output $name) -PathType Leaf)) { throw "Expected output is missing: $name" }
    }

    $probeRoot = Join-Path $temp 'probe'
    $null = New-Item -ItemType Directory -Path $probeRoot
    $source = Join-Path $probeRoot 'main.go'
    [IO.File]::WriteAllText($source, "package main`nfunc main() {}`n", [Text.UTF8Encoding]::new($false))
    $oldModule = $env:GO111MODULE
    try {
        $env:GO111MODULE = 'off'
        foreach ($resource in @(
            @{ Input = (Join-Path $output 'graphlayer-investigate.syso'); Name = 'graphlayer-investigate.syso'; Description = 'GraphLayer Investigation Application'; Internal = 'graphlayer-investigate'; Original = 'graphlayer-investigate.exe' },
            @{ Input = (Join-Path $output 'graphlayer-engine.res'); Name = 'graphlayer-engine.syso'; Description = 'GraphLayer Rust Analysis Engine'; Internal = 'graphlayer-engine'; Original = 'graphlayer-engine.exe' }
        )) {
            $package = Join-Path $probeRoot ([IO.Path]::GetFileNameWithoutExtension($resource.Name))
            $null = New-Item -ItemType Directory -Path $package
            Copy-Item -LiteralPath $source -Destination (Join-Path $package 'main.go')
            if ($resource.Name -eq 'graphlayer-engine.syso') {
                $workerSyso = Join-Path $package $resource.Name
                & $cvtres /nologo /MACHINE:X64 "/OUT:$workerSyso" $resource.Input
                if ($LASTEXITCODE -ne 0) { throw 'cvtres.exe could not convert the Rust .res resource for PE validation.' }
            } else {
                Copy-Item -LiteralPath $resource.Input -Destination (Join-Path $package $resource.Name)
            }
            $probeExe = Join-Path $package 'probe.exe'
            Push-Location $package
            try { & $go.Source build -o $probeExe . 2>&1 | Out-String | Write-Verbose }
            finally { Pop-Location }
            if ($LASTEXITCODE -ne 0 -or -not (Test-Path -LiteralPath $probeExe -PathType Leaf)) { throw 'Go could not link the resource object into a PE validation probe.' }
            Assert-VersionInfo $probeExe '12.34.56' $resource.Description $resource.Internal $resource.Original
        }
    } finally {
        $env:GO111MODULE = $oldModule
    }
    Write-Output 'Windows version resource contracts passed.'
} finally {
    Remove-TestDirectory $temp
}
