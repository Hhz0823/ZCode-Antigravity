param(
    [Parameter(Mandatory = $true)]
    [string]$SelfPath,
    [string]$Mode,
    [string]$ExtractTarget
)

$ErrorActionPreference = 'Stop'
[Console]::InputEncoding = [Text.UTF8Encoding]::new($false)
[Console]::OutputEncoding = [Text.UTF8Encoding]::new($false)

$packageVersion = '__PACKAGE_VERSION__'
$expectedPayloadSha256 = '__PAYLOAD_SHA256__'
$expectedManagerSha256 = '__MANAGER_SHA256__'
$expectedControlCenterSha256 = '__CONTROL_CENTER_SHA256__'
$expectedBackendSha256 = '__BACKEND_SHA256__'
$deploymentStageRoot = $null

function Write-Step([string]$Message) {
    Write-Host "`n==> $Message" -ForegroundColor Cyan
}

function Show-Result([string]$Message, [bool]$IsError = $false) {
    try {
        Add-Type -AssemblyName System.Windows.Forms
        $icon = if ($IsError) { [Windows.Forms.MessageBoxIcon]::Error } else { [Windows.Forms.MessageBoxIcon]::Information }
        [void][Windows.Forms.MessageBox]::Show($Message, 'ZCode Antigravity', [Windows.Forms.MessageBoxButtons]::OK, $icon)
    }
    catch {
        Write-Host $Message
    }
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

function Test-LocalTcpPort([int]$Port) {
    $client = [Net.Sockets.TcpClient]::new()
    try {
        $pending = $client.BeginConnect('127.0.0.1', $Port, $null, $null)
        if (-not $pending.AsyncWaitHandle.WaitOne(1500, $false)) {
            return $false
        }
        $client.EndConnect($pending)
        return $client.Connected
    }
    catch {
        return $false
    }
    finally {
        $client.Dispose()
    }
}

function Test-V2rayNTunUp {
    $pattern = '(?i)tun|wintun|v2ray|xray|sing.?box'
    try {
        $adapters = @(Get-NetAdapter -IncludeHidden -ErrorAction Stop | Where-Object {
            $_.Status -eq 'Up' -and (($_.Name + ' ' + $_.InterfaceDescription) -match $pattern)
        })
        if ($adapters.Count -gt 0) {
            return $true
        }
    }
    catch {
        $adapters = @()
    }
    try {
        $fallback = @(Get-CimInstance Win32_NetworkAdapter -ErrorAction Stop | Where-Object {
            $_.NetEnabled -eq $true -and (($_.Name + ' ' + $_.Description) -match $pattern)
        })
        return $fallback.Count -gt 0
    }
    catch {
        return $false
    }
}

function Expand-VerifiedPayload([string]$Destination) {
    $selfFull = [IO.Path]::GetFullPath($SelfPath)
    if (-not (Test-Path -LiteralPath $selfFull -PathType Leaf)) {
        throw "找不到单文件安装器: $selfFull"
    }
    $raw = [IO.File]::ReadAllText($selfFull, [Text.Encoding]::UTF8)
    $beginMarker = '#<ZCAB-' + 'PAYLOAD-BEGIN>'
    $endMarker = '#<ZCAB-' + 'PAYLOAD-END>'
    $begin = $raw.IndexOf($beginMarker, [StringComparison]::Ordinal)
    $end = $raw.IndexOf($endMarker, [StringComparison]::Ordinal)
    if ($begin -lt 0 -or $end -le $begin) {
        throw '安装器载荷标记缺失，文件可能已损坏。'
    }
    $begin += $beginMarker.Length
    $payloadText = $raw.Substring($begin, $end - $begin) -replace '\s', ''
    try {
        $payloadBytes = [Convert]::FromBase64String($payloadText)
    }
    catch {
        throw "安装器载荷不是有效 Base64，文件可能已损坏: $($_.Exception.Message)"
    }
    $destinationFull = [IO.Path]::GetFullPath($Destination)
    $root = [IO.Path]::GetPathRoot($destinationFull)
    if ([string]::Equals($destinationFull.TrimEnd('\'), $root.TrimEnd('\'), [StringComparison]::OrdinalIgnoreCase)) {
        throw "拒绝解包到磁盘根目录: $destinationFull"
    }
    New-Item -ItemType Directory -Path $destinationFull -Force | Out-Null

    $tempRoot = [IO.Path]::GetFullPath([IO.Path]::GetTempPath())
    $stage = Join-Path $tempRoot ('zcode-antigravity-oneclick-' + [Guid]::NewGuid().ToString('N'))
    $stageFull = [IO.Path]::GetFullPath($stage)
    if (-not $stageFull.StartsWith($tempRoot, [StringComparison]::OrdinalIgnoreCase)) {
        throw '临时目录超出系统临时目录。'
    }
    New-Item -ItemType Directory -Path $stageFull | Out-Null
    $zipPath = Join-Path $stageFull 'payload.zip'
    $expanded = Join-Path $stageFull 'expanded'
    try {
        [IO.File]::WriteAllBytes($zipPath, $payloadBytes)
        $sha = Get-Sha256 $zipPath
        if ($sha -ne $expectedPayloadSha256) {
            throw "安装器载荷校验失败；expected=$expectedPayloadSha256 actual=$sha"
        }
        Expand-Archive -LiteralPath $zipPath -DestinationPath $expanded -Force
        Copy-Item -Path (Join-Path $expanded '*') -Destination $destinationFull -Recurse -Force
    }
    finally {
        if (Test-Path -LiteralPath $stageFull) {
            Remove-Item -LiteralPath $stageFull -Recurse -Force
        }
    }

    return Confirm-VerifiedPackage $destinationFull
}

function Confirm-VerifiedPackage([string]$PackageDir) {
    $packageFull = [IO.Path]::GetFullPath($PackageDir)
    $manager = Join-Path $packageFull 'ZCode-Antigravity.exe'
    $controlCenter = Join-Path $packageFull 'ZCode-Antigravity-ControlCenter.exe'
    $backend = Join-Path $packageFull 'backend\cli-proxy-api.exe'
    foreach ($check in @(
        @{ Path = $manager; Expected = $expectedManagerSha256 },
        @{ Path = $controlCenter; Expected = $expectedControlCenterSha256 },
        @{ Path = $backend; Expected = $expectedBackendSha256 }
    )) {
        if (-not (Test-Path -LiteralPath $check.Path -PathType Leaf)) {
            throw "解包后缺少文件: $($check.Path)"
        }
        $actual = Get-Sha256 $check.Path
        if ($actual -ne $check.Expected) {
            throw "解包后文件校验失败: $($check.Path)"
        }
    }
    return $manager
}

function Install-VerifiedPackage([string]$Source, [string]$Destination) {
    $sourceFull = [IO.Path]::GetFullPath($Source)
    $destinationFull = [IO.Path]::GetFullPath($Destination)
    [void](Confirm-VerifiedPackage $sourceFull)
    $root = [IO.Path]::GetPathRoot($destinationFull)
    if ([string]::Equals($destinationFull.TrimEnd('\'), $root.TrimEnd('\'), [StringComparison]::OrdinalIgnoreCase)) {
        throw "拒绝部署到磁盘根目录: $destinationFull"
    }
    New-Item -ItemType Directory -Path $destinationFull -Force | Out-Null
    try {
        Copy-Item -Path (Join-Path $sourceFull '*') -Destination $destinationFull -Recurse -Force
    }
    catch {
        throw "部署新版本失败。已在复制前尝试安全停止旧网关；请确认没有安全软件占用安装目录后重试: $($_.Exception.Message)"
    }
    return Confirm-VerifiedPackage $destinationFull
}

try {
    if ($env:OS -ne 'Windows_NT' -or -not [Environment]::Is64BitOperatingSystem) {
        throw '本安装器仅支持 64 位 Windows。'
    }

    if ($Mode -eq '--extract-only') {
        if ([string]::IsNullOrWhiteSpace($ExtractTarget)) {
            throw '用法: 安装器.bat --extract-only <空目录>'
        }
        $targetFull = [IO.Path]::GetFullPath($ExtractTarget)
        if (Test-Path -LiteralPath $targetFull) {
            $existing = @(Get-ChildItem -LiteralPath $targetFull -Force -ErrorAction Stop)
            if ($existing.Count -gt 0) {
                throw "--extract-only 目标目录必须为空: $targetFull"
            }
        }
        Write-Step '校验并解包内嵌运行包'
        $manager = Expand-VerifiedPayload $targetFull
        $versionText = (& $manager version 2>&1 | Out-String).Trim()
        if ($LASTEXITCODE -ne 0 -or $versionText -notmatch [regex]::Escape($packageVersion)) {
            throw "管理器版本验证失败: $versionText"
        }
        Write-Host "SINGLE_BAT_EXTRACT_OK version=$packageVersion target=$targetFull" -ForegroundColor Green
        exit 0
    }

    if ([string]::IsNullOrWhiteSpace($env:LOCALAPPDATA)) {
        throw 'LOCALAPPDATA 未设置，无法确定当前用户安装目录。'
    }
    $proxyPort = 10808
    if (-not [string]::IsNullOrWhiteSpace($env:ZCODE_ANTIGRAVITY_PROXY_PORT)) {
        $parsedPort = 0
        if (-not [int]::TryParse($env:ZCODE_ANTIGRAVITY_PROXY_PORT, [ref]$parsedPort) -or $parsedPort -lt 1 -or $parsedPort -gt 65535) {
            throw 'ZCODE_ANTIGRAVITY_PROXY_PORT 必须是 1-65535 的端口号。'
        }
        $proxyPort = $parsedPort
    }

    Write-Step '检查 v2rayN TUN 与本地代理'
    if (-not (Test-V2rayNTunUp)) {
        throw '没有检测到处于 Up 状态的 TUN/Wintun/Xray/sing-box 适配器。请先在 v2rayN 开启 TUN 模式，再重新运行本 BAT。'
    }
    if (-not (Test-LocalTcpPort $proxyPort)) {
        throw "127.0.0.1:$proxyPort 未监听。请启动 v2rayN；若端口不是 10808，请先设置环境变量 ZCODE_ANTIGRAVITY_PROXY_PORT。"
    }

    if (@(Get-Process -Name ZCode -ErrorAction SilentlyContinue).Count -gt 0) {
        throw '检测到 ZCode 仍在运行。请从系统托盘右键 ZCode 并选择“退出”，然后重新运行本 BAT；不会强制结束 ZCode。'
    }

    $installRoot = Join-Path $env:LOCALAPPDATA 'ZCodeAntigravity'
    $installDir = Join-Path $installRoot ('app-' + $packageVersion)
    $tempRoot = [IO.Path]::GetFullPath([IO.Path]::GetTempPath())
    $deploymentStageRoot = [IO.Path]::GetFullPath((Join-Path $tempRoot ('zcode-antigravity-deploy-' + [Guid]::NewGuid().ToString('N'))))
    if (-not $deploymentStageRoot.StartsWith($tempRoot, [StringComparison]::OrdinalIgnoreCase)) {
        throw '部署暂存目录超出系统临时目录。'
    }
    $stagedPackage = Join-Path $deploymentStageRoot 'package'
    Write-Step '在隔离临时目录校验安装包'
    $stagedManager = Expand-VerifiedPayload $stagedPackage

    $statePath = Join-Path $installRoot 'state.json'
    if (Test-Path -LiteralPath $statePath) {
        try {
            $state = Get-Content -LiteralPath $statePath -Encoding UTF8 -Raw | ConvertFrom-Json
            if ([int]$state.pid -gt 0 -and $null -ne (Get-Process -Id ([int]$state.pid) -ErrorAction SilentlyContinue)) {
                Write-Step '在部署前安全停止旧版本网关'
                & $stagedManager stop
                if ($LASTEXITCODE -ne 0) {
                    throw "旧网关停止失败，退出码 $LASTEXITCODE"
                }
            }
        }
        catch {
            throw "读取或处理旧网关状态失败: $($_.Exception.Message)"
        }
    }

    Write-Step "安装到当前用户目录 $installDir"
    $manager = Install-VerifiedPackage $stagedPackage $installDir

    $settings = [ordered]@{
        preferredPort = 18080
        portScanEnd = 18180
        callbackPreferredPort = 51121
        callbackScanEnd = 51221
        proxyURL = "http://127.0.0.1:$proxyPort"
    }
    $settingsPath = Join-Path $installDir 'settings.json'
    $settingsJson = ($settings | ConvertTo-Json -Depth 3) + "`r`n"
    [IO.File]::WriteAllText($settingsPath, $settingsJson, [Text.UTF8Encoding]::new($false))

    $controlCenter = Join-Path $installDir 'ZCode-Antigravity-ControlCenter.exe'
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
        $shortcut.TargetPath = $controlCenter
        $shortcut.WorkingDirectory = $installDir
        $shortcut.Description = 'ZCode Antigravity 模型与额度控制中心'
        $shortcut.Save()
    }

    Start-Process -FilePath $controlCenter -ArgumentList '--auto-setup' -WindowStyle Hidden
    exit 0
}
catch {
    Write-Host "`n安装失败: $($_.Exception.Message)" -ForegroundColor Red
    Write-Host '没有删除账号、聊天、项目或其它 ZCode Provider。' -ForegroundColor Yellow
    Show-Result ("安装失败：`r`n`r`n" + $_.Exception.Message + "`r`n`r`n没有删除账号、聊天、项目或其它 ZCode Provider。") $true
    exit 1
}
finally {
    if (-not [string]::IsNullOrWhiteSpace($deploymentStageRoot)) {
        $tempRoot = [IO.Path]::GetFullPath([IO.Path]::GetTempPath())
        $stageFull = [IO.Path]::GetFullPath($deploymentStageRoot)
        if ($stageFull.StartsWith($tempRoot, [StringComparison]::OrdinalIgnoreCase) -and
            -not [string]::Equals($stageFull.TrimEnd('\'), $tempRoot.TrimEnd('\'), [StringComparison]::OrdinalIgnoreCase) -and
            (Test-Path -LiteralPath $stageFull -PathType Container)) {
            Remove-Item -LiteralPath $stageFull -Recurse -Force
        }
    }
}
