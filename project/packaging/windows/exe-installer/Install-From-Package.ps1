param(
    [Parameter(Mandatory = $true)][string]$PackageDir,
    [Parameter(Mandatory = $true)][string]$PackageVersion,
    [Parameter(Mandatory = $true)][string]$ExpectedManagerSha256,
    [Parameter(Mandatory = $true)][string]$ExpectedControlCenterSha256,
    [Parameter(Mandatory = $true)][string]$ExpectedBackendSha256,
    [string]$InstallRootOverride,
    [switch]$SkipEnvironmentPreflight,
    [switch]$SkipShortcuts
)

$ErrorActionPreference = 'Stop'
[Console]::InputEncoding = [Text.UTF8Encoding]::new($false)
[Console]::OutputEncoding = [Text.UTF8Encoding]::new($false)

function Write-Step([string]$Message) {
    Write-Output ("STEP: " + $Message)
}

function Get-Sha256([string]$Path) {
    $stream = [IO.File]::OpenRead($Path)
    $algorithm = [Security.Cryptography.SHA256]::Create()
    try {
        return ([BitConverter]::ToString($algorithm.ComputeHash($stream))).Replace('-', '').ToUpperInvariant()
    }
    finally {
        $algorithm.Dispose()
        $stream.Dispose()
    }
}

function Confirm-VerifiedPackage([string]$Directory) {
    $full = [IO.Path]::GetFullPath($Directory)
    $checks = @(
        @{ Path = (Join-Path $full 'ZCode-Antigravity.exe'); Expected = $ExpectedManagerSha256 },
        @{ Path = (Join-Path $full 'ZCode-Antigravity-ControlCenter.exe'); Expected = $ExpectedControlCenterSha256 },
        @{ Path = (Join-Path $full 'backend\cli-proxy-api.exe'); Expected = $ExpectedBackendSha256 }
    )
    foreach ($check in $checks) {
        if (-not (Test-Path -LiteralPath $check.Path -PathType Leaf)) {
            throw "安装包缺少文件: $($check.Path)"
        }
        if ((Get-Sha256 $check.Path) -ne $check.Expected.ToUpperInvariant()) {
            throw "安装包文件 SHA-256 校验失败: $($check.Path)"
        }
    }
    return $checks[0].Path
}

function Test-LocalTcpPort([int]$Port) {
    $client = [Net.Sockets.TcpClient]::new()
    try {
        $pending = $client.BeginConnect('127.0.0.1', $Port, $null, $null)
        if (-not $pending.AsyncWaitHandle.WaitOne(1500, $false)) { return $false }
        $client.EndConnect($pending)
        return $client.Connected
    }
    catch { return $false }
    finally { $client.Dispose() }
}

function Test-V2rayNTunUp {
    $pattern = '(?i)tun|wintun|v2ray|xray|sing.?box'
    try {
        $adapters = @(Get-NetAdapter -IncludeHidden -ErrorAction Stop | Where-Object {
            $_.Status -eq 'Up' -and (($_.Name + ' ' + $_.InterfaceDescription) -match $pattern)
        })
        if ($adapters.Count -gt 0) { return $true }
    }
    catch { }
    try {
        $fallback = @(Get-CimInstance Win32_NetworkAdapter -ErrorAction Stop | Where-Object {
            $_.NetEnabled -eq $true -and (($_.Name + ' ' + $_.Description) -match $pattern)
        })
        return $fallback.Count -gt 0
    }
    catch { return $false }
}

if ($env:OS -ne 'Windows_NT' -or -not [Environment]::Is64BitOperatingSystem) {
    throw '本安装器仅支持 64 位 Windows。'
}
if ([string]::IsNullOrWhiteSpace($env:LOCALAPPDATA) -and [string]::IsNullOrWhiteSpace($InstallRootOverride)) {
    throw 'LOCALAPPDATA 未设置，无法确定当前用户安装目录。'
}

$packageFull = [IO.Path]::GetFullPath($PackageDir)
Write-Step '校验内嵌运行包中的三个可执行文件'
$stagedManager = Confirm-VerifiedPackage $packageFull

$proxyPort = 10808
if (-not [string]::IsNullOrWhiteSpace($env:ZCODE_ANTIGRAVITY_PROXY_PORT)) {
    $parsedPort = 0
    if (-not [int]::TryParse($env:ZCODE_ANTIGRAVITY_PROXY_PORT, [ref]$parsedPort) -or $parsedPort -lt 1 -or $parsedPort -gt 65535) {
        throw 'ZCODE_ANTIGRAVITY_PROXY_PORT 必须是 1-65535 的端口号。'
    }
    $proxyPort = $parsedPort
}

if (-not $SkipEnvironmentPreflight) {
    Write-Step '检查 v2rayN TUN 与本地代理'
    if (-not (Test-V2rayNTunUp)) {
        throw '没有检测到处于 Up 状态的 TUN/Wintun/Xray/sing-box 适配器。请先在 v2rayN 开启 TUN 模式。'
    }
    if (-not (Test-LocalTcpPort $proxyPort)) {
        throw "127.0.0.1:$proxyPort 未监听。请启动 v2rayN，或设置 ZCODE_ANTIGRAVITY_PROXY_PORT。"
    }
    if (@(Get-Process -Name ZCode -ErrorAction SilentlyContinue).Count -gt 0) {
        throw '检测到 ZCode 仍在运行。请从系统托盘右键 ZCode 并选择“退出”；安装器不会强制结束 ZCode。'
    }
}

$installRoot = if ([string]::IsNullOrWhiteSpace($InstallRootOverride)) {
    Join-Path $env:LOCALAPPDATA 'ZCodeAntigravity'
}
else {
    [IO.Path]::GetFullPath($InstallRootOverride)
}
$installRoot = [IO.Path]::GetFullPath($installRoot)
$root = [IO.Path]::GetPathRoot($installRoot)
if ([string]::Equals($installRoot.TrimEnd('\'), $root.TrimEnd('\'), [StringComparison]::OrdinalIgnoreCase)) {
    throw '拒绝把安装根目录设置为磁盘根目录。'
}
$installDir = Join-Path $installRoot ('app-' + $PackageVersion)
$targetControlCenter = Join-Path $installDir 'ZCode-Antigravity-ControlCenter.exe'

if (Test-Path -LiteralPath $targetControlCenter -PathType Leaf) {
    $targetFull = [IO.Path]::GetFullPath($targetControlCenter)
    $running = @(Get-CimInstance Win32_Process -ErrorAction SilentlyContinue | Where-Object {
        -not [string]::IsNullOrWhiteSpace($_.ExecutablePath) -and
        [string]::Equals([IO.Path]::GetFullPath($_.ExecutablePath), $targetFull, [StringComparison]::OrdinalIgnoreCase)
    })
    if ($running.Count -gt 0) {
        throw '控制中心仍在运行。请先关闭“ZCode Antigravity 控制中心”，然后重新运行安装器。'
    }
}

$statePath = Join-Path $installRoot 'state.json'
if (Test-Path -LiteralPath $statePath -PathType Leaf) {
    try {
        $state = Get-Content -LiteralPath $statePath -Encoding UTF8 -Raw | ConvertFrom-Json
        if ([int]$state.pid -gt 0 -and $null -ne (Get-Process -Id ([int]$state.pid) -ErrorAction SilentlyContinue)) {
            Write-Step '按状态 PID 与实际路径安全停止旧 Bridge'
            $oldDataDir = $env:ZCODE_ANTIGRAVITY_DATA_DIR
            try {
                $env:ZCODE_ANTIGRAVITY_DATA_DIR = $installRoot
                & $stagedManager stop
                if ($LASTEXITCODE -ne 0) { throw "旧 Bridge 停止失败，退出码 $LASTEXITCODE" }
            }
            finally {
                if ($null -eq $oldDataDir) { Remove-Item Env:ZCODE_ANTIGRAVITY_DATA_DIR -ErrorAction SilentlyContinue }
                else { $env:ZCODE_ANTIGRAVITY_DATA_DIR = $oldDataDir }
            }
        }
    }
    catch {
        throw "读取或处理旧 Bridge 状态失败: $($_.Exception.Message)"
    }
}

Write-Step "安装到当前用户目录 $installDir"
New-Item -ItemType Directory -Path $installDir -Force | Out-Null
try {
    Copy-Item -Path (Join-Path $packageFull '*') -Destination $installDir -Recurse -Force
}
catch {
    throw "部署新版本失败；请确认安全软件没有占用安装目录: $($_.Exception.Message)"
}
[void](Confirm-VerifiedPackage $installDir)

$settings = [ordered]@{
    preferredPort = 18080
    portScanEnd = 18180
    callbackPreferredPort = 51121
    callbackScanEnd = 51221
    proxyURL = "http://127.0.0.1:$proxyPort"
}
[IO.File]::WriteAllText(
    (Join-Path $installDir 'settings.json'),
    (($settings | ConvertTo-Json -Depth 3) + "`r`n"),
    [Text.UTF8Encoding]::new($false)
)

if (-not $SkipShortcuts) {
    Write-Step '创建桌面和开始菜单快捷方式'
    $shortcutTargets = @()
    $desktop = [Environment]::GetFolderPath('Desktop')
    if (-not [string]::IsNullOrWhiteSpace($desktop)) {
        $shortcutTargets += (Join-Path $desktop 'ZCode Antigravity 控制中心.lnk')
    }
    if (-not [string]::IsNullOrWhiteSpace($env:APPDATA)) {
        $startMenuDir = Join-Path $env:APPDATA 'Microsoft\Windows\Start Menu\Programs\ZCode Antigravity'
        New-Item -ItemType Directory -Path $startMenuDir -Force | Out-Null
        $shortcutTargets += (Join-Path $startMenuDir 'ZCode Antigravity 控制中心.lnk')
    }
    $shell = New-Object -ComObject WScript.Shell
    foreach ($shortcutPath in $shortcutTargets) {
        $shortcut = $shell.CreateShortcut($shortcutPath)
        $shortcut.TargetPath = $targetControlCenter
        $shortcut.WorkingDirectory = $installDir
        $shortcut.Description = 'ZCode Antigravity 模型与额度控制中心'
        $shortcut.Save()
    }
}

Write-Output ("INSTALLED_CONTROL_CENTER=" + $targetControlCenter)
