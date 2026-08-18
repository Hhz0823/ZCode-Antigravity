param(
    [Parameter(Mandatory = $true)][string]$PackageDir,
    [Parameter(Mandatory = $true)][string]$OutputFile,
    [string]$PackageVersion = '0.6.0-test'
)

$ErrorActionPreference = 'Stop'
$packageFull = [IO.Path]::GetFullPath($PackageDir)
$outputFull = [IO.Path]::GetFullPath($OutputFile)
$templateDir = Join-Path $PSScriptRoot 'exe-installer'
$mainTemplate = Join-Path $templateDir 'main.go.template'
$encodingSource = Join-Path $templateDir 'encoding.go'
$installScript = Join-Path $templateDir 'Install-From-Package.ps1'
$manager = Join-Path $packageFull 'ZCode-Antigravity.exe'
$controlCenter = Join-Path $packageFull 'ZCode-Antigravity-ControlCenter.exe'
$backend = Join-Path $packageFull 'backend\cli-proxy-api.exe'
foreach ($required in @($mainTemplate, $encodingSource, $installScript, $manager, $controlCenter, $backend)) {
    if (-not (Test-Path -LiteralPath $required -PathType Leaf)) {
        throw "Missing EXE installer build input: $required"
    }
}

$forbiddenNames = @('local-api-key', 'state.json', 'config.yaml', 'last-smoke-test.json')
$forbidden = @(Get-ChildItem -LiteralPath $packageFull -Recurse -File | Where-Object {
    $_.Name -in $forbiddenNames -or $_.Name -like 'antigravity-*.json' -or $_.Name -like 'xai-*.json' -or $_.Extension -eq '.log'
})
if ($forbidden.Count -gt 0) {
    throw "Package contains runtime credentials/state/logs: $($forbidden.FullName -join ', ')"
}

$tempRoot = [IO.Path]::GetFullPath([IO.Path]::GetTempPath())
$buildRoot = [IO.Path]::GetFullPath((Join-Path $tempRoot ('zcode-antigravity-exe-build-' + [Guid]::NewGuid().ToString('N'))))
if (-not $buildRoot.StartsWith($tempRoot, [StringComparison]::OrdinalIgnoreCase) -or
    [string]::Equals($buildRoot.TrimEnd('\'), $tempRoot.TrimEnd('\'), [StringComparison]::OrdinalIgnoreCase)) {
    throw 'EXE installer build directory escaped the system temp directory.'
}

try {
    New-Item -ItemType Directory -Path $buildRoot | Out-Null
    $payloadPath = Join-Path $buildRoot 'payload.zip'
    Compress-Archive -Path (Join-Path $packageFull '*') -DestinationPath $payloadPath -CompressionLevel Optimal

    $payloadSha = (Get-FileHash -LiteralPath $payloadPath -Algorithm SHA256).Hash.ToUpperInvariant()
    $managerSha = (Get-FileHash -LiteralPath $manager -Algorithm SHA256).Hash.ToUpperInvariant()
    $controlCenterSha = (Get-FileHash -LiteralPath $controlCenter -Algorithm SHA256).Hash.ToUpperInvariant()
    $backendSha = (Get-FileHash -LiteralPath $backend -Algorithm SHA256).Hash.ToUpperInvariant()

    $main = [IO.File]::ReadAllText($mainTemplate, [Text.Encoding]::UTF8)
    $main = $main.Replace('__PACKAGE_VERSION__', $PackageVersion)
    $main = $main.Replace('__PAYLOAD_SHA256__', $payloadSha)
    $main = $main.Replace('__MANAGER_SHA256__', $managerSha)
    $main = $main.Replace('__CONTROL_CENTER_SHA256__', $controlCenterSha)
    $main = $main.Replace('__BACKEND_SHA256__', $backendSha)
    if ($main -match '__[A-Z0-9_]+__') {
        throw "Unresolved EXE installer placeholder: $($Matches[0])"
    }
    $mainSource = Join-Path $buildRoot 'main.go'
    $encodingBuildSource = Join-Path $buildRoot 'encoding.go'
    [IO.File]::WriteAllText($mainSource, $main, [Text.UTF8Encoding]::new($false))
    Copy-Item -LiteralPath $encodingSource -Destination $encodingBuildSource
    $installText = [IO.File]::ReadAllText($installScript, [Text.Encoding]::UTF8)
    [void][ScriptBlock]::Create($installText)
    $embeddedInstallScript = Join-Path $buildRoot 'install.ps1'
    [IO.File]::WriteAllText($embeddedInstallScript, $installText, [Text.UTF8Encoding]::new($true))
    $embeddedBytes = [IO.File]::ReadAllBytes($embeddedInstallScript)
    if ($embeddedBytes.Length -lt 3 -or $embeddedBytes[0] -ne 0xEF -or $embeddedBytes[1] -ne 0xBB -or $embeddedBytes[2] -ne 0xBF) {
        throw 'Embedded Windows PowerShell script is missing its UTF-8 BOM.'
    }

    $parent = Split-Path -Parent $outputFull
    if (-not [string]::IsNullOrWhiteSpace($parent)) {
        New-Item -ItemType Directory -Path $parent -Force | Out-Null
    }
    $oldCGO = $env:CGO_ENABLED
    $oldGOOS = $env:GOOS
    $oldGOARCH = $env:GOARCH
    try {
        $env:CGO_ENABLED = '0'
        $env:GOOS = 'windows'
        $env:GOARCH = 'amd64'
        & go build -trimpath -ldflags '-s -w -H windowsgui' -o $outputFull $mainSource $encodingBuildSource
        if ($LASTEXITCODE -ne 0) { throw "go build failed with exit code $LASTEXITCODE" }
    }
    finally {
        if ($null -eq $oldCGO) { Remove-Item Env:CGO_ENABLED -ErrorAction SilentlyContinue } else { $env:CGO_ENABLED = $oldCGO }
        if ($null -eq $oldGOOS) { Remove-Item Env:GOOS -ErrorAction SilentlyContinue } else { $env:GOOS = $oldGOOS }
        if ($null -eq $oldGOARCH) { Remove-Item Env:GOARCH -ErrorAction SilentlyContinue } else { $env:GOARCH = $oldGOARCH }
    }

    [pscustomobject]@{
        Output = $outputFull
        Version = $PackageVersion
        Bytes = (Get-Item -LiteralPath $outputFull).Length
        Sha256 = (Get-FileHash -LiteralPath $outputFull -Algorithm SHA256).Hash.ToUpperInvariant()
        PayloadSha256 = $payloadSha
        ManagerSha256 = $managerSha
        ControlCenterSha256 = $controlCenterSha
        BackendSha256 = $backendSha
    } | ConvertTo-Json -Compress
}
finally {
    if (Test-Path -LiteralPath $buildRoot -PathType Container) {
        $resolved = [IO.Path]::GetFullPath($buildRoot)
        if ($resolved.StartsWith($tempRoot, [StringComparison]::OrdinalIgnoreCase) -and
            -not [string]::Equals($resolved.TrimEnd('\'), $tempRoot.TrimEnd('\'), [StringComparison]::OrdinalIgnoreCase)) {
            Remove-Item -LiteralPath $resolved -Recurse -Force
        }
    }
}
