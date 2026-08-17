@echo off
setlocal
cd /d "%~dp0"
chcp 65001 >nul
"ZCode-Antigravity.exe" login-grok
set "RC=%ERRORLEVEL%"
echo.
pause
exit /b %RC%
