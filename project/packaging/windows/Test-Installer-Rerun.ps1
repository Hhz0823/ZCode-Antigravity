param(
    [Parameter(Mandatory = $true)]
    [string]$PackageDir,
    [Parameter(Mandatory = $true)]
    [string]$InstallerSource
)

$ErrorActionPreference = 'Stop'
$packageFull = [IO.Path]::GetFullPath($PackageDir)
$installerFull = [IO.Path]::GetFullPath($InstallerSource)
$managerName = 'ZCode-Antigravity.exe'
$controlCenterName = 'ZCode-Antigravity-ControlCenter.exe'
$backendRelative = 'backend\cli-proxy-api.exe'
foreach ($required in @(
    (Join-Path $packageFull $managerName),
    (Join-Path $packageFull $controlCenterName),
    (Join-Path $packageFull $backendRelative),
    $installerFull
)) {
    if (-not (Test-Path -LiteralPath $required -PathType Leaf)) {
        throw "Missing test input: $required"
    }
}

$installerText = [IO.File]::ReadAllText($installerFull, [Text.Encoding]::UTF8)
$stageIndex = $installerText.IndexOf('$stagedManager = Expand-VerifiedPayload $stagedPackage', [StringComparison]::Ordinal)
$stopIndex = $installerText.IndexOf('& $stagedManager stop', [StringComparison]::Ordinal)
$deployIndex = $installerText.IndexOf('$manager = Install-VerifiedPackage $stagedPackage $installDir', [StringComparison]::Ordinal)
if ($stageIndex -lt 0 -or $stopIndex -le $stageIndex -or $deployIndex -le $stopIndex) {
    throw 'Installer order regression: expected stage -> safe stop -> deploy.'
}
if ($installerText.Contains('Expand-VerifiedPayload $installDir')) {
    throw 'Installer regression: payload is expanded directly over the live install directory.'
}

$tempRoot = [IO.Path]::GetFullPath([IO.Path]::GetTempPath())
$testRoot = [IO.Path]::GetFullPath((Join-Path $tempRoot ('zcode-antigravity-rerun-test-' + [Guid]::NewGuid().ToString('N'))))
if (-not $testRoot.StartsWith($tempRoot, [StringComparison]::OrdinalIgnoreCase)) {
    throw 'Test directory escaped the system temp directory.'
}
$installed = Join-Path $testRoot 'installed'
$staged = Join-Path $testRoot 'staged'
$dataDir = Join-Path $testRoot 'data'
$authDir = Join-Path $dataDir 'auth'
$logsDir = Join-Path $dataDir 'logs'
$backendProcess = $null
$oldDataDir = $env:ZCODE_ANTIGRAVITY_DATA_DIR

try {
    foreach ($dir in @($installed, $staged, $authDir, $logsDir)) {
        New-Item -ItemType Directory -Path $dir -Force | Out-Null
    }
    Copy-Item -Path (Join-Path $packageFull '*') -Destination $installed -Recurse -Force
    Copy-Item -Path (Join-Path $packageFull '*') -Destination $staged -Recurse -Force

    $listener = [Net.Sockets.TcpListener]::new([Net.IPAddress]::Loopback, 0)
    $listener.Start()
    $port = ([Net.IPEndPoint]$listener.LocalEndpoint).Port
    $listener.Stop()

    $apiKey = 'installer-rerun-test-local-key-0000000000000000'
    $configPath = Join-Path $dataDir 'config.yaml'
    $authYaml = $authDir.Replace('\', '/')
    $config = @"
host: "127.0.0.1"
port: $port
remote-management:
  allow-remote: false
  secret-key: ""
  disable-control-panel: true
auth-dir: "$authYaml"
api-keys:
  - "$apiKey"
logging-to-file: false
request-log: false
usage-statistics-enabled: false
plugins:
  enabled: false
  dir: "plugins"
  configs: {}
"@
    [IO.File]::WriteAllText($configPath, $config, [Text.UTF8Encoding]::new($false))
    [IO.File]::WriteAllText((Join-Path $dataDir 'local-api-key'), $apiKey + "`n", [Text.UTF8Encoding]::new($false))

    $installedBackend = Join-Path $installed $backendRelative
    $backendProcess = Start-Process -FilePath $installedBackend -ArgumentList @('-config', $configPath, '-local-model') -WorkingDirectory $dataDir -WindowStyle Hidden -PassThru
    $deadline = (Get-Date).AddSeconds(20)
    do {
        Start-Sleep -Milliseconds 200
        try {
            $healthy = (Invoke-WebRequest -UseBasicParsing -Uri "http://127.0.0.1:$port/healthz" -TimeoutSec 2).StatusCode -eq 200
        }
        catch {
            $healthy = $false
        }
    } while (-not $healthy -and -not $backendProcess.HasExited -and (Get-Date) -lt $deadline)
    if (-not $healthy) {
        throw 'Isolated backend did not become healthy.'
    }

    $state = [ordered]@{
        port = $port
        pid = $backendProcess.Id
        backendPath = $installedBackend
        launcherVersion = 'rerun-test'
    }
    [IO.File]::WriteAllText((Join-Path $dataDir 'state.json'), (($state | ConvertTo-Json -Depth 3) + "`n"), [Text.UTF8Encoding]::new($false))

    $lockReproduced = $false
    try {
        Copy-Item -LiteralPath (Join-Path $staged $backendRelative) -Destination $installedBackend -Force -ErrorAction Stop
    }
    catch {
        $lockReproduced = $_.Exception.Message -match 'being used by another process|cannot access the file|正在由另一进程使用|无法访问'
    }
    if (-not $lockReproduced) {
        throw 'The pre-fix locked-backend failure was not reproduced.'
    }

    $env:ZCODE_ANTIGRAVITY_DATA_DIR = $dataDir
    $stagedManager = Join-Path $staged $managerName
    & $stagedManager stop
    if ($LASTEXITCODE -ne 0) {
        throw "Staged manager failed to stop the recorded backend: $LASTEXITCODE"
    }
    $backendProcess.Refresh()
    if (-not $backendProcess.HasExited) {
        if (-not $backendProcess.WaitForExit(5000)) {
            throw 'Recorded backend remained alive after the staged-manager stop.'
        }
    }

    Copy-Item -Path (Join-Path $staged '*') -Destination $installed -Recurse -Force
    foreach ($relative in @($managerName, $controlCenterName, $backendRelative)) {
        $expected = (Get-FileHash -LiteralPath (Join-Path $staged $relative) -Algorithm SHA256).Hash
        $actual = (Get-FileHash -LiteralPath (Join-Path $installed $relative) -Algorithm SHA256).Hash
        if ($actual -ne $expected) {
            throw "Post-stop deployment hash mismatch: $relative"
        }
    }

    [pscustomobject]@{
        Result = 'INSTALLER_RERUN_LOCK_TEST_OK'
        LockFailureReproduced = $lockReproduced
        StagedManagerStoppedRecordedBackend = $true
        SameVersionDeploymentAfterStop = $true
    } | ConvertTo-Json -Compress
}
finally {
    if ($null -ne $backendProcess -and -not $backendProcess.HasExited) {
        Stop-Process -Id $backendProcess.Id -Force -ErrorAction SilentlyContinue
        [void]$backendProcess.WaitForExit(5000)
    }
    if ($null -eq $oldDataDir) {
        Remove-Item Env:ZCODE_ANTIGRAVITY_DATA_DIR -ErrorAction SilentlyContinue
    }
    else {
        $env:ZCODE_ANTIGRAVITY_DATA_DIR = $oldDataDir
    }
    $resolved = [IO.Path]::GetFullPath($testRoot)
    if ($resolved.StartsWith($tempRoot, [StringComparison]::OrdinalIgnoreCase) -and
        -not [string]::Equals($resolved.TrimEnd('\'), $tempRoot.TrimEnd('\'), [StringComparison]::OrdinalIgnoreCase) -and
        (Test-Path -LiteralPath $resolved -PathType Container)) {
        Remove-Item -LiteralPath $resolved -Recurse -Force
    }
}
