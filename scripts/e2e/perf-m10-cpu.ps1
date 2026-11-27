# M10 T09 / AC-MAT-9: idle CPU with the window minimized.
# Same method as the T00 baseline (spikes/m10-effects/cpu/cpu-baseline.ps1):
# launch, wait for the main window, minimize it, let it settle, then sum the
# CPU time of the whole process tree (the app plus its WebView2 children) over
# a fixed window. Prints one JSON object and stops the tree it started.
#
# Only ever point this at a temp APPDATA; perf-m10.sh sets it up.
param(
    [Parameter(Mandatory = $true)][string]$AppPath,
    [Parameter(Mandatory = $true)][string]$AppData,
    [int]$Seconds = 300,
    [int]$SettleSeconds = 60
)

$ErrorActionPreference = "Stop"

if (-not ("M10PerfWin32" -as [type])) {
    Add-Type -TypeDefinition @"
using System;
using System.Runtime.InteropServices;
public static class M10PerfWin32 {
    [DllImport("user32.dll")] public static extern bool ShowWindow(IntPtr hWnd, int nCmdShow);
    [DllImport("user32.dll")] public static extern bool IsIconic(IntPtr hWnd);
}
"@
}

$env:APPDATA = $AppData
$proc = Start-Process -FilePath $AppPath -PassThru

$deadline = (Get-Date).AddSeconds(90)
while ((Get-Date) -lt $deadline) {
    Start-Sleep -Milliseconds 500
    $proc.Refresh()
    if ($proc.MainWindowHandle -ne [IntPtr]::Zero) { break }
}
if ($proc.MainWindowHandle -eq [IntPtr]::Zero) {
    Stop-Process -Id $proc.Id -Force -ErrorAction SilentlyContinue
    throw "no main window after 90 s"
}

[M10PerfWin32]::ShowWindow($proc.MainWindowHandle, 6) | Out-Null  # SW_MINIMIZE
Start-Sleep -Seconds $SettleSeconds
$proc.Refresh()
$minimized = [M10PerfWin32]::IsIconic($proc.MainWindowHandle)

function Get-TreeIds([int]$RootId) {
    $all = Get-CimInstance Win32_Process -Property ProcessId, ParentProcessId
    $set = New-Object 'System.Collections.Generic.HashSet[int]'
    [void]$set.Add($RootId)
    $changed = $true
    while ($changed) {
        $changed = $false
        foreach ($p in $all) {
            if ($set.Contains([int]$p.ParentProcessId) -and -not $set.Contains([int]$p.ProcessId)) {
                [void]$set.Add([int]$p.ProcessId)
                $changed = $true
            }
        }
    }
    return $set
}

function Get-CpuMap($Ids) {
    $map = @{}
    $rows = @()
    foreach ($id in $Ids) {
        $p = Get-Process -Id $id -ErrorAction SilentlyContinue
        if ($p) {
            $cpu = $p.TotalProcessorTime.TotalSeconds
            $map[[int]$id] = $cpu
            $rows += [pscustomobject]@{ pid = [int]$id; name = $p.ProcessName; cpuSeconds = [math]::Round($cpu, 3) }
        }
    }
    return @{ map = $map; rows = $rows }
}

$ids0 = Get-TreeIds $proc.Id
$start = Get-CpuMap $ids0
$t0 = Get-Date
Start-Sleep -Seconds $Seconds
$t1 = Get-Date
$ids1 = Get-TreeIds $proc.Id
$end = Get-CpuMap $ids1

# A PID missing at t0 contributes its whole CPU time, which over-states rather
# than under-states the measurement (same rule as the T00 baseline).
$delta = 0.0
foreach ($id in $end.map.Keys) {
    $before = 0.0
    if ($start.map.ContainsKey($id)) { $before = $start.map[$id] }
    $delta += ($end.map[$id] - $before)
}
$elapsed = ($t1 - $t0).TotalSeconds

$result = [pscustomobject]@{
    app           = $AppPath
    minimized     = $minimized
    settleSeconds = $SettleSeconds
    seconds       = [math]::Round($elapsed, 2)
    cpuSeconds    = [math]::Round($delta, 3)
    cpuPercentOne = [math]::Round(100.0 * $delta / $elapsed, 3)
    processCount  = $end.map.Count
    processes     = $end.rows
    startedAt     = $t0.ToString("o")
}
$result | ConvertTo-Json -Depth 5

foreach ($id in $end.map.Keys) { Stop-Process -Id $id -Force -ErrorAction SilentlyContinue }
Start-Sleep -Seconds 2
foreach ($id in $end.map.Keys) { Stop-Process -Id $id -Force -ErrorAction SilentlyContinue }
