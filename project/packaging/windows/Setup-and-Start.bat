@echo off
setlocal
cd /d "%~dp0"
chcp 65001 >nul
start "" "ZCode-Antigravity-ControlCenter.exe" --auto-setup
exit /b 0
