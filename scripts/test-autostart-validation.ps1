# Read and test the production validation functions without registering tasks.
$ErrorActionPreference = 'Stop'
$source = Join-Path (Split-Path -Parent $PSScriptRoot) 'src/autostart.ps1'
$tokens = $null
$errors = $null
$ast = [Management.Automation.Language.Parser]::ParseFile($source, [ref]$tokens, [ref]$errors)
if ($errors) { throw ($errors | Out-String) }
foreach ($name in @('Resolve-TaskUserSid', 'Test-Task')) {
    $function = $ast.Find({ param($node) $node -is [Management.Automation.Language.FunctionDefinitionAst] -and $node.Name -eq $name }, $true)
    if (-not $function) { throw "Missing production function: $name" }
    . ([scriptblock]::Create($function.Extent.Text))
}
$sid = [Security.Principal.WindowsIdentity]::GetCurrent().User.Value
$account = [Security.Principal.WindowsIdentity]::GetCurrent().Name
$Executable = 'C:\Clash Tray\ClashTray.exe'
$principal = [pscustomobject]@{ UserId = $account; RunLevel = 1; LogonType = 3 }
$action = [pscustomobject]@{ Type = 0; Path = $Executable; Arguments = $null; WorkingDirectory = 'C:\Clash Tray' }
$trigger = [pscustomobject]@{ Type = 9; Enabled = $true; UserId = $account }
$actions = [pscustomobject]@{ Count = 1; Entry = $action }
$actions | Add-Member ScriptMethod Item { param($index) return $this.Entry }
$triggers = [pscustomobject]@{ Count = 1; Entry = $trigger }
$triggers | Add-Member ScriptMethod Item { param($index) return $this.Entry }
$task = [pscustomobject]@{ Enabled = $true; Definition = [pscustomobject]@{
    Principal = $principal; Actions = $actions; Triggers = $triggers
    Settings = [pscustomobject]@{ ExecutionTimeLimit = 'PT0S'; DisallowStartIfOnBatteries = $false; StopIfGoingOnBatteries = $false }
} }
if (-not (Test-Task $task)) { throw "Account name / null arguments rejected: $script:taskValidationError" }
$principal.UserId = $sid
$trigger.UserId = $sid
$action.Arguments = ''
if (-not (Test-Task $task)) { throw 'SID / empty arguments rejected' }
$principal.UserId = 'S-1-5-18'
if (Test-Task $task) { throw 'Wrong account accepted' }
$principal.UserId = $sid
$trigger.UserId = 'S-1-5-18'
if (Test-Task $task) { throw 'Wrong logon user accepted' }
$trigger.UserId = $sid
$principal.RunLevel = 0
if (Test-Task $task) { throw 'Non-elevated task accepted' }
$principal.RunLevel = 1
$action.Arguments = '--unexpected'
if (Test-Task $task) { throw 'Unexpected arguments accepted' }
$action.Arguments = ''
$action.Path = 'C:\other.exe'
if (Test-Task $task) { throw 'Wrong executable accepted' }
$action.Path = $Executable
$task.Enabled = $false
if (Test-Task $task) { throw 'Disabled task accepted' }
Write-Output 'PASS: 8 task-validation cases; no scheduled tasks modified.'
