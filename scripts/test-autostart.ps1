#Requires -RunAsAdministrator
$ErrorActionPreference = 'Stop'
$projectRoot = Split-Path -Parent $PSScriptRoot
$script = Join-Path $projectRoot 'src/autostart.ps1'
$powerShell = Join-Path $env:SystemRoot 'System32/WindowsPowerShell/v1.0/powershell.exe'
$testDirectory = Join-Path ([IO.Path]::GetTempPath()) ('Clash Tray 测试 & ' + [guid]::NewGuid().ToString())
New-Item -ItemType Directory -Path $testDirectory | Out-Null
$executable = Join-Path $testDirectory 'Clash Tray.exe'
Copy-Item -LiteralPath (Join-Path $env:SystemRoot 'System32/notepad.exe') -Destination $executable
$taskName = 'ClashTray-Test-' + [guid]::NewGuid().ToString()
$service = New-Object -ComObject 'Schedule.Service'
$service.Connect()
$folder = $service.GetFolder('\')
function Invoke-Setting([string]$mode, [string]$path = $executable) {
    $output = & $powerShell -NoProfile -NonInteractive -ExecutionPolicy Bypass -File $script -Mode $mode -Executable $path -TaskName $taskName
    if ($LASTEXITCODE -ne 0) { throw "Autostart $mode failed" }
    return "$output"
}
try {
    if ((Invoke-Setting 'disable') -ne 'disabled') { throw 'Disable missing task failed' }
    if ((Invoke-Setting 'enable') -ne 'enabled') { throw 'Enable verification failed' }
    if ((Invoke-Setting 'query') -ne 'enabled') { throw 'Readback failed' }
    $definition = $folder.GetTask($taskName).Definition
    if ($definition.Principal.RunLevel -ne 1 -or $definition.Principal.LogonType -ne 3) { throw 'Wrong privilege or session' }
    if ($definition.Triggers.Item(1).Delay -ne 'PT10S') { throw 'Missing login delay' }
    if ((Invoke-Setting 'query' (Join-Path $env:SystemRoot 'System32/cmd.exe')) -ne 'disabled') { throw 'Mismatched executable accepted' }
    $folder.GetTask($taskName).Enabled = $false
    if ((Invoke-Setting 'query') -ne 'disabled') { throw 'Disabled task accepted' }
    if ((Invoke-Setting 'enable') -ne 'enabled') { throw 'Re-enable failed' }
    if ((Invoke-Setting 'disable') -ne 'disabled') { throw 'Delete failed' }
    if ((Invoke-Setting 'disable') -ne 'disabled') { throw 'Repeated delete failed' }
    Write-Output 'PASS: task registration, elevated interactive principal, login delay, path validation, enable/disable and removal.'
} finally {
    # Exact test-only GUID name; never deletes the production task.
    try { $folder.DeleteTask($taskName, 0) } catch {}
    # Only remove the two exact fixture paths; no recursive cleanup needed.
    Remove-Item -LiteralPath $executable -Force
    Remove-Item -LiteralPath $testDirectory -Force
}
