<#
.SYNOPSIS
    dnspick 安装脚本（Windows / PowerShell）

.DESCRIPTION
    自动检测 CPU 架构，下载对应的预编译版本并安装到用户目录，加入 PATH。

.EXAMPLE
    irm https://raw.githubusercontent.com/palemoky/dnspick/main/install.ps1 | iex

.NOTES
    可选环境变量：
      DNSPICK_VERSION   指定版本（默认 latest），例如 v2.0.0
      DNSPICK_BIN_DIR   安装目录（默认 %LOCALAPPDATA%\Programs\dnspick）
#>

$ErrorActionPreference = 'Stop'

$Repo = 'palemoky/dnspick'
$App  = 'dnspick'
$Version = if ($env:DNSPICK_VERSION) { $env:DNSPICK_VERSION } else { 'latest' }

function Info($m) { Write-Host "==> $m" -ForegroundColor Blue }
function Ok($m)   { Write-Host "[OK] $m" -ForegroundColor Green }
function Warn($m) { Write-Host "[!] $m"  -ForegroundColor Yellow }
function Die($m)  { Write-Host "[x] $m"  -ForegroundColor Red; exit 1 }

# 1. 检测 CPU 架构
switch ($env:PROCESSOR_ARCHITECTURE) {
    'AMD64' { $Arch = 'amd64' }
    'ARM64' { $Arch = 'arm64' }
    'x86'   { Die '不支持 32 位 (x86) 系统' }
    default { Die "不支持的架构：$($env:PROCESSOR_ARCHITECTURE)" }
}

# 2. 拼接下载地址
$Asset = "$App-windows-$Arch.zip"
$Url = if ($Version -eq 'latest') {
    "https://github.com/$Repo/releases/latest/download/$Asset"
} else {
    "https://github.com/$Repo/releases/download/$Version/$Asset"
}

Info "平台：windows/$Arch，版本：$Version"

# 3. 下载并解压到临时目录
$Tmp = Join-Path $env:TEMP ("dnspick-" + [System.Guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Path $Tmp -Force | Out-Null
try {
    $ZipPath = Join-Path $Tmp $Asset
    Info "下载 $Asset ..."
    try {
        Invoke-WebRequest -Uri $Url -OutFile $ZipPath -UseBasicParsing
    } catch {
        Die "下载失败：$Url"
    }

    Info "解压 ..."
    Expand-Archive -Path $ZipPath -DestinationPath $Tmp -Force

    # 归档内二进制名为 dnspick-windows-<arch>.exe
    $BinSrc = Join-Path $Tmp "$App-windows-$Arch.exe"
    if (-not (Test-Path $BinSrc)) { $BinSrc = Join-Path $Tmp "$App.exe" }
    if (-not (Test-Path $BinSrc)) { Die '归档中未找到可执行文件' }

    # 4. 选择安装目录
    $BinDir = if ($env:DNSPICK_BIN_DIR) {
        $env:DNSPICK_BIN_DIR
    } else {
        Join-Path $env:LOCALAPPDATA "Programs\$App"
    }
    New-Item -ItemType Directory -Path $BinDir -Force | Out-Null

    $Dest = Join-Path $BinDir "$App.exe"
    Move-Item -Path $BinSrc -Destination $Dest -Force
    Ok "已安装到 $Dest"

    # 5. 加入用户 PATH
    $UserPath = [Environment]::GetEnvironmentVariable('Path', 'User')
    if (($UserPath -split ';') -notcontains $BinDir) {
        [Environment]::SetEnvironmentVariable('Path', "$UserPath;$BinDir", 'User')
        $env:Path = "$env:Path;$BinDir"
        Warn "已将 $BinDir 加入用户 PATH，请重启终端后生效。"
    }

    Write-Host "完成！" -ForegroundColor Green -NoNewline
    Write-Host " 运行 " -NoNewline
    Write-Host "$App --help" -ForegroundColor Blue -NoNewline
    Write-Host " 开始使用。"
}
finally {
    Remove-Item -Path $Tmp -Recurse -Force -ErrorAction SilentlyContinue
}
