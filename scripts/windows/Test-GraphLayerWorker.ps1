[CmdletBinding()]
param(
    [Parameter(Mandatory)] [string] $PackageDirectory,
    [Parameter(Mandatory)] [ValidateSet('Development', 'Signed')] [string] $PackageKind,
    [string] $ExpectedSignerThumbprint,
    [string] $SignToolPath
)

$ErrorActionPreference = 'Stop'
$packageArguments = @{ PackageDirectory = $PackageDirectory }
if ($PackageKind -eq 'Signed') {
    $packageArguments.RequireSigned = $true
    $packageArguments.ExpectedSignerThumbprint = $ExpectedSignerThumbprint
    $packageArguments.SignToolPath = $SignToolPath
}
& (Join-Path $PSScriptRoot 'Test-GraphLayerWindowsPackage.ps1') @packageArguments | Out-Null

# This calls the same one-shot protocol as the application, with no device data.
# There is no alternate executable, shell launch, or analysis implementation.
$workerPath = Join-Path (Resolve-Path -LiteralPath $PackageDirectory).Path 'graphlayer-engine.exe'
$start = [Diagnostics.ProcessStartInfo]::new()
$start.FileName = $workerPath
$start.UseShellExecute = $false
$start.CreateNoWindow = $true
$start.RedirectStandardInput = $true
$start.RedirectStandardOutput = $true
$start.RedirectStandardError = $true
$worker = [Diagnostics.Process]::new()
$worker.StartInfo = $start
$workerStarted = $false
try {
    try { $workerStarted = $worker.Start() }
    catch {
        throw 'The verified worker could not start. Check Code Integrity event 3077 and its correlated 3089 signature events; package verification does not change Windows policy.'
    }
    $worker.StandardInput.WriteLine('{"protocol_version":"graphlayer-engine-v1","operation":"path.is_asa_vendor","input":{"vendor":"cisco-asa"}}')
    $worker.StandardInput.Close()
    # The response is tiny. Waiting before reading also bounds a noisy worker:
    # it fills the redirected pipe and reaches this timeout instead of growing memory.
    if (-not $worker.WaitForExit(5000)) { throw 'Worker protocol probe timed out.' }
    if ($worker.ExitCode -ne 0) { throw 'Worker protocol probe failed.' }
    $outputBuffer = [char[]]::new(4097)
    $outputLength = $worker.StandardOutput.ReadBlock($outputBuffer, 0, $outputBuffer.Length)
    $errorBuffer = [char[]]::new(1025)
    $errorLength = $worker.StandardError.ReadBlock($errorBuffer, 0, $errorBuffer.Length)
    if ($outputLength -gt 4096 -or $errorLength -ne 0) { throw 'Worker returned unexpected protocol output.' }
    $lines = @([string]::new($outputBuffer, 0, $outputLength).Trim() -split '\r?\n')
    if ($lines.Count -ne 1) { throw 'Worker returned more than one protocol response.' }
    $response = $lines[0] | ConvertFrom-Json
    if ($response.protocol_version -ne 'graphlayer-engine-v1' -or
        $response.operation -ne 'path.is_asa_vendor' -or
        $response.result -isnot [bool] -or $response.result -ne $true -or
        $response.PSObject.Properties.Name -contains 'error') {
        throw 'Worker returned an incompatible protocol response.'
    }
    Write-Output "Verified $PackageKind Rust worker protocol: graphlayer-engine-v1"
} finally {
    if ($workerStarted -and -not $worker.HasExited) {
        try { $worker.Kill($true) }
        catch [InvalidOperationException] { if (-not $worker.HasExited) { throw } }
        [void]$worker.WaitForExit(1000)
    }
    $worker.Dispose()
}
