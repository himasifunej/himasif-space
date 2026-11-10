param(
    [Parameter(Mandatory=$true)][string]$Executable,
    [string]$Out = 'build/island-spike-results.json',
    [string[]]$Modes = @('baseline','hidden','pill'),
    [int]$WarmupSeconds = 10,
    [int]$SampleSeconds = 20
)
$ErrorActionPreference = 'Stop'
if ($WarmupSeconds + $SampleSeconds -gt 40) { throw 'Spike exits at 45 s; keep warmup + samples <=40 s.' }
$exePath = (Resolve-Path -LiteralPath $Executable).Path
$results = @()
foreach ($mode in $Modes) {
    $spikeProcess = Start-Process -FilePath $exePath -ArgumentList "--island-spike=$mode" -WindowStyle Hidden -PassThru
    try {
        Start-Sleep -Seconds $WarmupSeconds
        $spikeProcess.Refresh()
        if ($spikeProcess.HasExited) { throw "Spike $mode exited early: $($spikeProcess.ExitCode)" }
        $cpuStart = $spikeProcess.TotalProcessorTime.TotalSeconds
        $watch = [Diagnostics.Stopwatch]::StartNew()
        $workingSet = @()
        $privateBytes = @()
        for ($sample = 0; $sample -lt $SampleSeconds; $sample++) {
            Start-Sleep -Seconds 1
            $spikeProcess.Refresh()
            if ($spikeProcess.HasExited) { throw "Spike $mode exited while sampling" }
            $workingSet += $spikeProcess.WorkingSet64
            $privateBytes += $spikeProcess.PrivateMemorySize64
        }
        $watch.Stop()
        $cpuDelta = $spikeProcess.TotalProcessorTime.TotalSeconds - $cpuStart
        $result = [pscustomobject]@{
            mode = $mode
            workingSetMiB = [math]::Round(($workingSet | Measure-Object -Average).Average / 1MB, 3)
            privateBytesMiB = [math]::Round(($privateBytes | Measure-Object -Average).Average / 1MB, 3)
            cpuSeconds = [math]::Round($cpuDelta, 6)
            elapsedSeconds = [math]::Round($watch.Elapsed.TotalSeconds, 3)
            cpuOneCorePercent = [math]::Round(100 * $cpuDelta / $watch.Elapsed.TotalSeconds, 4)
            cpuMachinePercent = [math]::Round(100 * $cpuDelta / $watch.Elapsed.TotalSeconds / [Environment]::ProcessorCount, 4)
        }
        $results += $result
        $result | ConvertTo-Json -Compress | Write-Output
        if (-not $spikeProcess.WaitForExit(20000)) { throw 'Spike did not close itself within 50 s' }
    } finally {
        # Only the isolated child launched above; never stop a running Space.
        if (-not $spikeProcess.HasExited) { $spikeProcess.Kill() }
        $spikeProcess.Dispose()
    }
}
$report = [pscustomobject]@{
    recordedAt = (Get-Date).ToString('o')
    os = [Environment]::OSVersion.VersionString
    logicalProcessors = [Environment]::ProcessorCount
    warmupSeconds = $WarmupSeconds
    sampleSeconds = $SampleSeconds
    results = $results
}
$parent = Split-Path -Parent $Out
if ($parent) { New-Item -ItemType Directory -Path $parent -Force | Out-Null }
$report | ConvertTo-Json -Depth 6 | Set-Content -LiteralPath $Out -Encoding utf8
