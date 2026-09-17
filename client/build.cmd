@echo off
rem Rebuilds FoghornClient.exe using the C# compiler that is part of Windows.
rem No Visual Studio or SDK needed. Run from this folder.
setlocal
set CSC=%WINDIR%\Microsoft.NET\Framework64\v4.0.30319\csc.exe
if not exist "%CSC%" set CSC=%WINDIR%\Microsoft.NET\Framework\v4.0.30319\csc.exe
if not exist "%CSC%" (echo Could not find csc.exe - is .NET Framework 4 present? & exit /b 1)
"%CSC%" /nologo /target:winexe /platform:anycpu /optimize+ /out:FoghornClient.exe /win32icon:foghorn.ico ^
  /r:System.dll /r:System.Core.dll /r:System.Drawing.dll /r:System.Windows.Forms.dll /r:System.Web.Extensions.dll ^
  src\*.cs
if errorlevel 1 exit /b 1
echo Built FoghornClient.exe
