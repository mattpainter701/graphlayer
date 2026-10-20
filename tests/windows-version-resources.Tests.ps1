param(
    [string] $RcPath,
    [string] $CvtresPath,
    [string] $RustWorkerPath,
    [string] $Version = '12.34.56'
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
    if (-not $go) { throw 'Go is required to validate the generated Go application resource.' }
    $cargo = $null
    if ([string]::IsNullOrWhiteSpace($RustWorkerPath)) {
        $cargo = Get-Command cargo.exe -ErrorAction SilentlyContinue
        if (-not $cargo) { throw 'Cargo is required to build the Rust worker resource test.' }
    }

    $output = Join-Path $temp 'resources'
    & $newResources -Version $Version -RcPath $rc -CvtresPath $cvtres -OutputDirectory $output | Out-Null
    foreach ($name in @('graphlayer-investigate.rc', 'graphlayer-engine.rc', 'graphlayer-engine.res', 'graphlayer-investigate.syso')) {
        if (-not (Test-Path -LiteralPath (Join-Path $output $name) -PathType Leaf)) { throw "Expected output is missing: $name" }
    }

    # Build the actual Rust worker with the worker .res. Microsoft LINK accepts
    # .res directly and invokes CVTRES; the Rust test must exercise that path,
    # rather than validating the resource through a surrogate Go executable.
    $workerResource = [IO.Path]::GetFullPath((Join-Path $output 'graphlayer-engine.res'))
    if ([string]::IsNullOrWhiteSpace($RustWorkerPath)) {
        $oldWorkerResource = $env:GRAPHLAYER_VERSION_RESOURCE
        $oldTargetDirectory = $env:CARGO_TARGET_DIR
        $buildLog = Join-Path $temp 'rust-worker-build.log'
        $targetTriple = $env:CARGO_BUILD_TARGET
        $targetDirectory = Join-Path $temp 'rust-target'
        $cargoArguments = @('build', '--locked', '--release', '-p', 'graphlayer-engine', '--bin', 'graphlayer-engine')
        if (-not [string]::IsNullOrWhiteSpace($targetTriple)) { $cargoArguments += @('--target', $targetTriple) }
        try {
            $env:GRAPHLAYER_VERSION_RESOURCE = $workerResource
            $env:CARGO_TARGET_DIR = $targetDirectory
            Push-Location $repo
            try {
                & $cargo.Source @cargoArguments *> $buildLog
                $cargoExitCode = $LASTEXITCODE
            } finally {
                Pop-Location
            }
            if ($cargoExitCode -ne 0) {
                throw "Rust worker resource build failed with exit code $cargoExitCode; compiler diagnostics were suppressed."
            }
            $workerOutputDirectory = Join-Path $targetDirectory 'release'
            if (-not [string]::IsNullOrWhiteSpace($targetTriple)) {
                $workerOutputDirectory = Join-Path $workerOutputDirectory $targetTriple
            }
            $RustWorkerPath = Join-Path $workerOutputDirectory 'graphlayer-engine.exe'
        } finally {
            $env:GRAPHLAYER_VERSION_RESOURCE = $oldWorkerResource
            $env:CARGO_TARGET_DIR = $oldTargetDirectory
        }
    }
    $workerExe = [IO.Path]::GetFullPath($RustWorkerPath)
    if (-not (Test-Path -LiteralPath $workerExe -PathType Leaf)) {
        throw 'The Rust worker executable for version-resource validation was not found.'
    }
    Assert-VersionInfo $workerExe $Version 'GraphLayer Rust Analysis Engine' 'graphlayer-engine' 'graphlayer-engine.exe'

    # The Go link probe covers only the application's .syso resource object.
    $probeRoot = Join-Path $temp 'go-probe'
    $null = New-Item -ItemType Directory -Path $probeRoot
    $package = Join-Path $probeRoot 'graphlayer-investigate'
    $null = New-Item -ItemType Directory -Path $package
    $source = Join-Path $package 'main.go'
    [IO.File]::WriteAllText($source, "package main`nfunc main() {}`n", [Text.UTF8Encoding]::new($false))
    Copy-Item -LiteralPath (Join-Path $output 'graphlayer-investigate.syso') -Destination (Join-Path $package 'graphlayer-investigate.syso')
    $oldModule = $env:GO111MODULE
    $goLog = Join-Path $temp 'go-resource-probe.log'
    try {
        $env:GO111MODULE = 'off'
        $probeExe = Join-Path $package 'probe.exe'
        Push-Location $package
        try {
            & $go.Source build -o $probeExe . *> $goLog
            $goExitCode = $LASTEXITCODE
        } finally {
            Pop-Location
        }
        if ($goExitCode -ne 0 -or -not (Test-Path -LiteralPath $probeExe -PathType Leaf)) {
            throw "Go could not link the application resource into a PE probe (exit code $goExitCode); compiler diagnostics were suppressed."
        }
        Assert-VersionInfo $probeExe $Version 'GraphLayer Investigation Application' 'graphlayer-investigate' 'graphlayer-investigate.exe'
    } finally {
        $env:GO111MODULE = $oldModule
    }
    Write-Output 'Windows version resource contracts passed.'
} finally {
    Remove-TestDirectory $temp
}
