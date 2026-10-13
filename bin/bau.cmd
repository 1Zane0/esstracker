@echo off
rem Baut das Programm unter Windows. Ohne Argument mit Fenster, mit
rem --ohne-fenster ohne.
rem
rem Anders als unter Linux braucht das Fenster hier weder C-Compiler noch
rem pkg-config: die Bindung an WebView2 ist reines Go.
setlocal
cd /d "%~dp0.."

set "schalter="
if "%~1"=="--ohne-fenster" set "schalter=-tags ohne_fenster"

go build %schalter% -o esstracker.exe .\cmd\esstracker
if errorlevel 1 exit /b 1

for %%D in (esstracker.exe) do echo -^> %%~fD, %%~zD Bytes
