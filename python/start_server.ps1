# 透過 venv 啟動 FastAPI HTTP server（Method B）
# 使用方式：在 python\ 目錄下執行 .\start_server.ps1

$ErrorActionPreference = "Stop"
$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
Set-Location $scriptDir

if (-not (Test-Path ".venv")) {
    Write-Error "找不到虛擬環境，請先執行 .\setup.ps1"
    exit 1
}

Write-Host "啟動 FastAPI server（Method B - HTTP）on port 8001 ..."
.venv\Scripts\uvicorn.exe http_server:app --port 8001 --reload
