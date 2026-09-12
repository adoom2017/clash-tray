param(
    [ValidateSet('query', 'enable', 'disable')][string]$Mode,
    [string]$Executable,
    [string]$TaskName
)
$ErrorActionPreference = 'Stop'
[Console]::OutputEncoding = [Text.UTF8Encoding]::new($false)
try {
    $sid = [Security.Principal.WindowsIdentity]::GetCurrent().User.Value
    if (-not $TaskName) { $TaskName = "ClashTray-$sid" }
    $service = New-Object -ComObject 'Schedule.Service'
    $service.Connect()
    $folder = $service.GetFolder('\')
    function Find-Task {
        try { return $folder.GetTask($TaskName) }
        catch {
            $errorCause = $_.Exception
            while ($null -ne $errorCause) {
                if ($errorCause.HResult -in @(-2147024894, -2147024893)) { return $null }
                $errorCause = $errorCause.InnerException
            }
            throw
        }
    }
    function Resolve-TaskUserSid([string]$identity) {
        if ([string]::IsNullOrWhiteSpace($identity)) { return $null }
        try {
            if ($identity -match '^S-1-') {
                return ([Security.Principal.SecurityIdentifier]::new($identity)).Value
            }
            return ([Security.Principal.NTAccount]::new($identity)).Translate([Security.Principal.SecurityIdentifier]).Value
        } catch { return $null }
    }
    function Test-Task($task) {
        $script:taskValidationError = 'Task is missing or disabled'
        if ($null -eq $task -or -not $task.Enabled) { return $false }
        $definition = $task.Definition
        $script:taskValidationError = "Principal mismatch: user=$($definition.Principal.UserId), runLevel=$($definition.Principal.RunLevel), logonType=$($definition.Principal.LogonType), actions=$($definition.Actions.Count)"
        if ((Resolve-TaskUserSid $definition.Principal.UserId) -ne $sid -or
            $definition.Principal.RunLevel -ne 1 -or
            $definition.Principal.LogonType -ne 3 -or
            $definition.Actions.Count -ne 1) { return $false }
        $action = $definition.Actions.Item(1)
        $script:taskValidationError = "Action mismatch: path=$($action.Path), arguments=$($action.Arguments), workingDirectory=$($action.WorkingDirectory)"
        if ($action.Type -ne 0 -or $action.Path -ine $Executable -or
            [string]$action.Arguments -ne '' -or
            $action.WorkingDirectory -ine (Split-Path -Parent $Executable)) { return $false }
        $settings = $definition.Settings
        $script:taskValidationError = "Settings mismatch: executionTimeLimit=$($settings.ExecutionTimeLimit), disallowBattery=$($settings.DisallowStartIfOnBatteries), stopBattery=$($settings.StopIfGoingOnBatteries)"
        if ($settings.ExecutionTimeLimit -ne 'PT0S' -or
            $settings.DisallowStartIfOnBatteries -or $settings.StopIfGoingOnBatteries) { return $false }
        $script:taskValidationError = 'No enabled logon trigger for the current user'
        for ($i = 1; $i -le $definition.Triggers.Count; $i++) {
            $trigger = $definition.Triggers.Item($i)
            if ($trigger.Type -eq 9 -and $trigger.Enabled -and (Resolve-TaskUserSid $trigger.UserId) -eq $sid) {
                $script:taskValidationError = ''
                return $true
            }
        }
        return $false
    }
    switch ($Mode) {
        'enable' {
            if (-not [IO.Path]::IsPathRooted($Executable) -or -not (Test-Path -LiteralPath $Executable -PathType Leaf)) {
                throw 'Executable must be an existing absolute file path'
            }
            $definition = $service.NewTask(0)
            $definition.RegistrationInfo.Description = 'Start Clash Tray with administrator privileges when this user logs on.'
            $definition.Principal.UserId = $sid
            $definition.Principal.LogonType = 3 # TASK_LOGON_INTERACTIVE_TOKEN: visible user session, no stored password.
            $definition.Principal.RunLevel = 1 # TASK_RUNLEVEL_HIGHEST
            $trigger = $definition.Triggers.Create(9) # TASK_TRIGGER_LOGON
            $trigger.UserId = $sid
            $trigger.Enabled = $true
            $trigger.Delay = 'PT10S'
            $definition.Settings.Enabled = $true
            $definition.Settings.StartWhenAvailable = $true
            $definition.Settings.DisallowStartIfOnBatteries = $false
            $definition.Settings.StopIfGoingOnBatteries = $false
            $definition.Settings.ExecutionTimeLimit = 'PT0S'
            $definition.Settings.MultipleInstances = 2 # TASK_INSTANCES_IGNORE_NEW
            $action = $definition.Actions.Create(0)
            $action.Path = $Executable
            $action.WorkingDirectory = Split-Path -Parent $Executable
            $null = $folder.RegisterTaskDefinition($TaskName, $definition, 6, $sid, $null, 3)
            if (-not (Test-Task (Find-Task))) { throw "Registered task failed verification: $script:taskValidationError" }
        }
        'disable' {
            if ($null -ne (Find-Task)) { $folder.DeleteTask($TaskName, 0) }
            if ($null -ne (Find-Task)) { throw 'Task still exists after deletion' }
        }
    }
    if (Test-Task (Find-Task)) { [Console]::Write('enabled') }
    else { [Console]::Write('disabled') }
} catch {
    [Console]::Error.Write($_.Exception.Message)
    exit 1
}
