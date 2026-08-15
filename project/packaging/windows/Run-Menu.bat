@echo off
setlocal
cd /d "%~dp0"
chcp 65001 >nul
"ZCode-Antigravity.exe" menu
set "RC=%ERRORLEVEL%"
if not "%RC%"=="0" echo.
if not "%RC%"=="0" echo Program exited with code %RC%.
pause
exit /b %RC%
