@echo off
setlocal
cd /d "%~dp0"
"%~dp0ZCode-Antigravity.exe" smoke claude-sonnet-4-6
set "EXITCODE=%ERRORLEVEL%"
echo.
if not "%EXITCODE%"=="0" echo Test failed. Enable Google Claude in Settings, then review the exact error and gateway log; do not share auth files.
pause
exit /b %EXITCODE%
