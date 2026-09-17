@echo off
powershell.exe -NoProfile -NonInteractive -ExecutionPolicy Bypass -File "%~dp0Uninstall-FoghornClient.ps1" %*
exit /b %ERRORLEVEL%
