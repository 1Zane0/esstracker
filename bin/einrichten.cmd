@echo off
rem Baut, legt das Programm nach %LOCALAPPDATA%\Programs\esstracker und traegt
rem es ins Startmenue ein - das Gegenstueck zu bin/einrichten.sh.
setlocal
cd /d "%~dp0.."

call bin\bau.cmd %*
if errorlevel 1 exit /b 1

set "ziel=%LOCALAPPDATA%\Programs\esstracker"
if not exist "%ziel%" mkdir "%ziel%"

rem Eine laufende exe laesst sich unter Windows nicht ueberschreiben.
copy /y esstracker.exe "%ziel%\esstracker.exe" >nul
if errorlevel 1 goto belegt
echo -^> installiert: %ziel%\esstracker.exe

"%ziel%\esstracker.exe" einrichten
if errorlevel 1 exit /b 1

echo.
echo Beim ersten Start fragt die Windows-Firewall, ob der Esstracker ins Netz darf.
echo Fuer private Netzwerke erlauben, sonst kommt das Handy nicht durch.
echo.
echo Zum Aufrufen per Namen im Terminal gehoert %ziel% in den PATH.
exit /b 0

:belegt
echo Kopieren nach %ziel% ging nicht. Laeuft der Esstracker noch? Fenster schliessen, dann noch einmal.
exit /b 1
