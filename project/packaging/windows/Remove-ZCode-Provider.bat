@echo off
setlocal
cd /d "%~dp0"
chcp 65001 >nul
"ZCode-Antigravity.exe" remove-provider
set "RC=%ERRORLEVEL%"
echo.
pause
exit /b %RC%
