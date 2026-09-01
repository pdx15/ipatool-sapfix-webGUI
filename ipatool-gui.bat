@echo off
title ipatool GUI — App Store Downloader for Windows
cd /d "%~dp0"

echo =======================================================
echo          ipatool GUI for Windows
echo =======================================================
echo.

:: Check if port 54321 is already in use
netstat -ano | findstr :54321 | findstr LISTENING >nul 2>nul
if %ERRORLEVEL% equ 0 (
    echo [ERROR] Port 54321 is already in use!
    echo.
    echo Another instance of ipatool GUI or another application
    echo is already using port 54321.
    echo.
    echo Please close the other application and try again,
    echo or wait a moment for it to fully shut down.
    echo.
    pause
    exit /b 1
)

echo Checking for ipatool binary...

if exist "ipatool.exe" (
    echo Found ipatool.exe. Starting GUI server...
    start "" "ipatool.exe" gui --port 54321
    exit /b 0
)

if exist "bin\ipatool.exe" (
    echo Found bin\ipatool.exe. Starting GUI server...
    start "" "bin\ipatool.exe" gui --port 54321
    exit /b 0
)

where ipatool >nul 2>nul
if %ERRORLEVEL% equ 0 (
    echo Found ipatool in PATH. Starting GUI server...
    start "" ipatool gui --port 54321
    exit /b 0
)

where python >nul 2>nul
if %ERRORLEVEL% equ 0 (
    echo Starting Python GUI Server...
    start "" python ipatool-gui.py --port 54321
    exit /b 0
)

where py >nul 2>nul
if %ERRORLEVEL% equ 0 (
    echo Starting Python GUI Server...
    start "" py ipatool-gui.py --port 54321
    exit /b 0
)

echo [ERROR] Neither ipatool.exe nor Python was found.
echo Please place ipatool.exe in this folder or install Python.
echo.
pause
