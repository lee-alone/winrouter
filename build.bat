@echo off
setlocal

set "PROJECT_ROOT=%~dp0"
set "BUILD_VERSION=%~1"
if not defined BUILD_VERSION set "BUILD_VERSION=0.1.0-dev"

echo Building WinRouter %BUILD_VERSION%...
powershell.exe -NoProfile -ExecutionPolicy Bypass -File "%PROJECT_ROOT%scripts\build.ps1" -Version "%BUILD_VERSION%"
if errorlevel 1 (
    echo.
    echo Build failed.
    exit /b 1
)

for %%F in (
    "%PROJECT_ROOT%build\bin\WinRouter.exe"
    "%PROJECT_ROOT%build\bin\WinRouter-helper.exe"
    "%PROJECT_ROOT%build\bin\WinRouter-core-smoke.exe"
) do (
    if not exist "%%~F" (
        echo Missing build artifact: %%~F
        exit /b 1
    )
)

echo.
echo Build completed. Artifacts:
echo   %PROJECT_ROOT%build\bin\WinRouter.exe
echo   %PROJECT_ROOT%build\bin\WinRouter-helper.exe
echo   %PROJECT_ROOT%build\bin\WinRouter-core-smoke.exe
echo   %PROJECT_ROOT%build\bin\resources\core\
exit /b 0
