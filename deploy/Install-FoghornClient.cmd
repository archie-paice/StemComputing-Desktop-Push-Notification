@echo off
rem Wrapper so the installer runs whatever the PC's PowerShell execution policy is.
rem Use THIS file as the Group Policy start-up script and put the settings in
rem "Script Parameters", e.g.   -ServerUrl http://foghorn01:8080 -ClientKey 0123abcd...
powershell.exe -NoProfile -NonInteractive -ExecutionPolicy Bypass -File "%~dp0Install-FoghornClient.ps1" %*
exit /b %ERRORLEVEL%
