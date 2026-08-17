param(
    [Parameter(Mandatory = $true)]
    [string]$PackageDir,
    [Parameter(Mandatory = $true)]
    [string]$OutputFile,
    [string]$PackageVersion = '0.4.6-test'
)

$ErrorActionPreference = 'Stop'
$packageFull = [IO.Path]::GetFullPath($PackageDir)
$outputFull = [IO.Path]::GetFullPath($OutputFile)
$runtimeScript = Join-Path $PSScriptRoot 'OneClick-Installer.ps1'
$manager = Join-Path $packageFull 'ZCode-Antigravity.exe'
$controlCenter = Join-Path $packageFull 'ZCode-Antigravity-ControlCenter.exe'
$backend = Join-Path $packageFull 'backend\cli-proxy-api.exe'

foreach ($required in @($runtimeScript, $manager, $controlCenter, $backend)) {
    if (-not (Test-Path -LiteralPath $required -PathType Leaf)) {
        throw "Missing required file: $required"
    }
}

$forbiddenNames = @('local-api-key', 'state.json', 'config.yaml', 'last-smoke-test.json')
$forbidden = @(Get-ChildItem -LiteralPath $packageFull -Recurse -File | Where-Object {
    $_.Name -in $forbiddenNames -or $_.Name -like 'antigravity-*.json' -or $_.Name -like 'xai-*.json' -or $_.Extension -eq '.log'
})
if ($forbidden.Count -gt 0) {
    throw "Package contains runtime credentials/state/logs: $($forbidden.FullName -join ', ')"
}

$managerSha = (Get-FileHash -LiteralPath $manager -Algorithm SHA256).Hash.ToUpperInvariant()
$controlCenterSha = (Get-FileHash -LiteralPath $controlCenter -Algorithm SHA256).Hash.ToUpperInvariant()
$backendSha = (Get-FileHash -LiteralPath $backend -Algorithm SHA256).Hash.ToUpperInvariant()
$tempRoot = [IO.Path]::GetFullPath([IO.Path]::GetTempPath())
$tempDir = Join-Path $tempRoot ('zcode-antigravity-build-' + [Guid]::NewGuid().ToString('N'))
$tempFull = [IO.Path]::GetFullPath($tempDir)
if (-not $tempFull.StartsWith($tempRoot, [StringComparison]::OrdinalIgnoreCase)) {
    throw 'Temporary build directory escaped the system temp directory.'
}
New-Item -ItemType Directory -Path $tempFull | Out-Null
$zipPath = Join-Path $tempFull 'payload.zip'

try {
    Add-Type -AssemblyName System.IO.Compression.FileSystem
    [IO.Compression.ZipFile]::CreateFromDirectory(
        $packageFull,
        $zipPath,
        [IO.Compression.CompressionLevel]::Optimal,
        $false
    )
    $payloadBytes = [IO.File]::ReadAllBytes($zipPath)
    $payloadSha = (Get-FileHash -LiteralPath $zipPath -Algorithm SHA256).Hash.ToUpperInvariant()
    $payloadBase64 = [Convert]::ToBase64String($payloadBytes)
    $payloadLines = ([regex]::Matches($payloadBase64, '.{1,76}') | ForEach-Object Value) -join "`r`n"

    $script = [IO.File]::ReadAllText($runtimeScript, [Text.Encoding]::UTF8)
    $script = $script.Replace('__PACKAGE_VERSION__', $PackageVersion)
    $script = $script.Replace('__PAYLOAD_SHA256__', $payloadSha)
    $script = $script.Replace('__MANAGER_SHA256__', $managerSha)
    $script = $script.Replace('__CONTROL_CENTER_SHA256__', $controlCenterSha)
    $script = $script.Replace('__BACKEND_SHA256__', $backendSha)
    if ($script -match '__[A-Z0-9_]+__') {
        throw "Unresolved installer placeholder: $($Matches[0])"
    }
    [void][ScriptBlock]::Create($script)

    $launcher = @'
@echo off
setlocal EnableExtensions DisableDelayedExpansion
chcp 65001 >nul
set "ZCAB_SELF=%~f0"
set "ZCAB_MODE=%~1"
set "ZCAB_TARGET=%~2"
if /I not "%ZCAB_MODE%"=="--extract-only" (
  start "" powershell.exe -NoLogo -NoProfile -WindowStyle Hidden -ExecutionPolicy Bypass -Command "$ErrorActionPreference='Stop';$raw=[IO.File]::ReadAllText($env:ZCAB_SELF,[Text.Encoding]::UTF8);$begin='#<ZCAB-'+'PS-BEGIN>';$end='#<ZCAB-'+'PS-END>';$i=$raw.IndexOf($begin,[StringComparison]::Ordinal);$j=$raw.IndexOf($end,[StringComparison]::Ordinal);if($i -lt 0 -or $j -le $i){throw 'Embedded PowerShell section is missing'};$i+=$begin.Length;$script=$raw.Substring($i,$j-$i);& ([ScriptBlock]::Create($script)) -SelfPath $env:ZCAB_SELF -Mode $env:ZCAB_MODE -ExtractTarget $env:ZCAB_TARGET"
  exit /b 0
)
powershell.exe -NoLogo -NoProfile -ExecutionPolicy Bypass -Command "$ErrorActionPreference='Stop';$raw=[IO.File]::ReadAllText($env:ZCAB_SELF,[Text.Encoding]::UTF8);$begin='#<ZCAB-'+'PS-BEGIN>';$end='#<ZCAB-'+'PS-END>';$i=$raw.IndexOf($begin,[StringComparison]::Ordinal);$j=$raw.IndexOf($end,[StringComparison]::Ordinal);if($i -lt 0 -or $j -le $i){throw 'Embedded PowerShell section is missing'};$i+=$begin.Length;$script=$raw.Substring($i,$j-$i);& ([ScriptBlock]::Create($script)) -SelfPath $env:ZCAB_SELF -Mode $env:ZCAB_MODE -ExtractTarget $env:ZCAB_TARGET;exit $LASTEXITCODE"
set "ZCAB_RC=%ERRORLEVEL%"
exit /b %ZCAB_RC%
'@
    $content = $launcher.TrimEnd() + "`r`n#<ZCAB-PS-BEGIN>`r`n" + $script.Trim() + "`r`n#<ZCAB-PS-END>`r`n#<ZCAB-PAYLOAD-BEGIN>`r`n" + $payloadLines + "`r`n#<ZCAB-PAYLOAD-END>`r`n"
    $parent = Split-Path -Parent $outputFull
    if (-not [string]::IsNullOrWhiteSpace($parent)) {
        New-Item -ItemType Directory -Path $parent -Force | Out-Null
    }
    [IO.File]::WriteAllText($outputFull, $content, [Text.UTF8Encoding]::new($false))

    [pscustomobject]@{
        Output = $outputFull
        Version = $PackageVersion
        Bytes = (Get-Item -LiteralPath $outputFull).Length
        Sha256 = (Get-FileHash -LiteralPath $outputFull -Algorithm SHA256).Hash.ToUpperInvariant()
        PayloadSha256 = $payloadSha
        ManagerSha256 = $managerSha
        ControlCenterSha256 = $controlCenterSha
        BackendSha256 = $backendSha
        PackageFiles = @(Get-ChildItem -LiteralPath $packageFull -Recurse -File).Count
    } | ConvertTo-Json -Compress
}
finally {
    if (Test-Path -LiteralPath $tempFull) {
        Remove-Item -LiteralPath $tempFull -Recurse -Force
    }
}
