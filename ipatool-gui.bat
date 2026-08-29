@echo off
title ipatool GUI — App Store Downloader for Windows
cd /d "%~dp0"

echo =======================================================
echo          ipatool GUI for Windows
echo =======================================================
echo.
echo Checking for ipatool binary...

if exist "ipatool.exe" (
    echo Found ipatool.exe. Starting GUI server...
    start "" "ipatool.exe" gui --port 8080
    exit /b 0
)

if exist "bin\ipatool.exe" (
    echo Found bin\ipatool.exe. Starting GUI server...
    start "" "bin\ipatool.exe" gui --port 8080
    exit /b 0
)

where ipatool >nul 2>nul
if %ERRORLEVEL% equ 0 (
    echo Found ipatool in PATH. Starting GUI server...
    start "" ipatool gui --port 8080
    exit /b 0
)

where python >nul 2>nul
if %ERRORLEVEL% equ 0 (
    echo Starting Python GUI Server...
    start "" python ipatool-gui.py --port 8080
    exit /b 0
)

where py >nul 2>nul
if %ERRORLEVEL% equ 0 (
    echo Starting Python GUI Server...
    start "" py ipatool-gui.py --port 8080
    exit /b 0
)

echo [ERROR] Neither ipatool.exe nor Python was found.
echo Please place ipatool.exe in this folder or install Python.
echo.
pause
