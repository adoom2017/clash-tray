# Run from any directory after cargo build --release --locked.
$ErrorActionPreference = 'Stop'
$projectRoot = Split-Path -Parent $PSScriptRoot
Push-Location $projectRoot
try {
    $metadata = cargo metadata --no-deps --format-version 1 --locked | ConvertFrom-Json
    if ($LASTEXITCODE -ne 0) { throw 'Unable to read Cargo metadata' }
    $version = ($metadata.packages | Where-Object name -eq 'clash-tray').version
    if ($version -notmatch '^\d+\.\d+\.\d+(?:[-+][0-9A-Za-z.-]+)?$') {
        throw "Invalid package version: $version"
    }
    $binary = Join-Path $metadata.target_directory 'release/ClashTray.exe'
    if (-not (Test-Path -LiteralPath $binary)) { throw 'Run cargo build --release --locked first' }
    $dist = Join-Path $projectRoot 'dist'
    New-Item -ItemType Directory -Force -Path $dist | Out-Null
    $name = "ClashTray-v$version-windows-x64"
    $stage = Join-Path $dist ([guid]::NewGuid().ToString())
    New-Item -ItemType Directory -Path $stage | Out-Null
    try {
        Copy-Item -LiteralPath $binary -Destination (Join-Path $stage 'ClashTray.exe')
        Copy-Item -LiteralPath 'README.md','LICENSE' -Destination $stage
        $iconDirectory = Join-Path $stage 'assets/icons'
        New-Item -ItemType Directory -Path $iconDirectory -Force | Out-Null
        Copy-Item -LiteralPath 'assets/icons/running.png','assets/icons/stopped.png' -Destination $iconDirectory
        $archive = Join-Path $dist "$name.zip"
        Compress-Archive -Path (Join-Path $stage '*') -DestinationPath $archive -Force
        $hash = (Get-FileHash -LiteralPath $archive -Algorithm SHA256).Hash.ToLowerInvariant()
        "$hash  $name.zip" | Set-Content -LiteralPath "$archive.sha256" -Encoding ascii
        Write-Output "Created $archive"
    } finally {
        $resolvedStage = [IO.Path]::GetFullPath($stage)
        if (-not $resolvedStage.StartsWith([IO.Path]::GetFullPath($dist) + [IO.Path]::DirectorySeparatorChar)) {
            throw "Invalid staging directory: $resolvedStage"
        }
        Remove-Item -LiteralPath $resolvedStage -Recurse -Force
    }
} finally {
    Pop-Location
}
