# ipatool GUI PowerShell Launcher for Windows
$ErrorActionPreference = "SilentlyContinue"

Write-Host "=======================================================" -ForegroundColor Cyan
Write-Host "         ipatool GUI for Windows" -ForegroundColor Cyan
Write-Host "=======================================================" -ForegroundColor Cyan
Write-Host ""

$CurrentDir = Split-Path -Parent $MyInvocation.MyCommand.Path
Set-Location $CurrentDir

if (Test-Path "$CurrentDir\ipatool.exe") {
    Write-Host "Found ipatool.exe. Launching GUI..." -ForegroundColor Green
    Start-Process "$CurrentDir\ipatool.exe" -ArgumentList "gui", "--port", "8080"
    exit 0
}

if (Test-Path "$CurrentDir\bin\ipatool.exe") {
    Write-Host "Found bin\ipatool.exe. Launching GUI..." -ForegroundColor Green
    Start-Process "$CurrentDir\bin\ipatool.exe" -ArgumentList "gui", "--port", "8080"
    exit 0
}

if (Get-Command "ipatool" -ErrorAction SilentlyContinue) {
    Write-Host "Found ipatool in PATH. Launching GUI..." -ForegroundColor Green
    Start-Process "ipatool" -ArgumentList "gui", "--port", "8080"
    exit 0
}

if (Get-Command "python" -ErrorAction SilentlyContinue) {
    Write-Host "Starting Python GUI server..." -ForegroundColor Green
    Start-Process "python" -ArgumentList "ipatool-gui.py", "--port", "8080"
    exit 0
}

Write-Host "[ERROR] Could not find ipatool.exe or Python in the folder." -ForegroundColor Red
Write-Host "Please place ipatool.exe in this folder or install Python." -ForegroundColor Yellow
Read-Host "Press Enter to exit..."
