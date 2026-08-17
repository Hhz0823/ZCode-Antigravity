@echo off
setlocal
cd /d "%~dp0"
"%~dp0ZCode-Antigravity.exe" smoke grok-build-0.1
set "EXITCODE=%ERRORLEVEL%"
echo.
if not "%EXITCODE%"=="0" echo Test failed. Review the exact error and gateway log; do not share auth files.
pause
exit /b %EXITCODE%
