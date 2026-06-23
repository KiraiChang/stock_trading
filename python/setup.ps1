# 建立 Python 虛擬環境並安裝依賴
# 使用方式：在 python\ 目錄下執行 .\setup.ps1

$ErrorActionPreference = "Stop"
$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
Set-Location $scriptDir

Write-Host ""
Write-Host "=== Trading Python Setup ===" -ForegroundColor Cyan
Write-Host "目錄：$scriptDir"
Write-Host ""

# 確認 python 可用
$pythonVersion = python --version 2>&1
Write-Host "[1/3] Python 版本：$pythonVersion"

# 建立 venv
if (Test-Path ".venv") {
    Write-Host "[2/3] 虛擬環境已存在，跳過建立"
} else {
    Write-Host "[2/3] 建立虛擬環境 .venv ..."
    python -m venv .venv
    Write-Host "      完成"
}

# 安裝套件
Write-Host "[3/3] 升級 pip ..."
.venv\Scripts\python.exe -m pip install --upgrade pip

Write-Host "[3/3] 安裝 requirements.txt ..."
.venv\Scripts\pip.exe install -r requirements.txt

Write-Host ""
Write-Host "=== 設定完成 ===" -ForegroundColor Green
Write-Host "啟動服務請執行："
Write-Host "  .\start_worker.ps1      # Method A：DB polling worker"
Write-Host "  .\start_server.ps1      # Method B：FastAPI HTTP server"
Write-Host ""
