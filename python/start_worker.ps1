# 透過 venv 啟動 DB polling worker（Method A）
# 使用方式：在 python\ 目錄下執行 .\start_worker.ps1

$ErrorActionPreference = "Stop"
$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
Set-Location $scriptDir

if (-not (Test-Path ".venv")) {
    Write-Error "找不到虛擬環境，請先執行 .\setup.ps1"
    exit 1
}

Write-Host "啟動 worker（Method A - DB polling）..."
.venv\Scripts\python.exe worker.py
