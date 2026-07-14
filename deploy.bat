@echo off
setlocal enabledelayedexpansion
chcp 65001 >nul 2>&1
cd /d "%~dp0"

echo.
echo ============================================
echo   GinChat Deploy
echo ============================================
echo.

echo [1/3] Building Go backend (Linux amd64)...

REM ---- Locate Go ----
set "GO_BIN="
for %%d in (
    "C:\Users\%USERNAME%\sdk\go1.26.1\bin"
    "C:\Users\%USERNAME%\sdk\go1.25.1\bin"
    "C:\Program Files\Go\bin"
    "D:\GoLang\bin"
) do (
    if exist "%%~d\go.exe" (
        if "!GO_BIN!"=="" set "GO_BIN=%%~d"
    )
)

if "!GO_BIN!"=="" (
    echo [FAIL] Go not found!
    pause
    exit /b 1
)
echo   Using: !GO_BIN!\go.exe

set "CGO_ENABLED=0"
set "GOOS=linux"
set "GOARCH=amd64"
set "GOTOOLCHAIN=local"

if not exist ".deploy" mkdir ".deploy"

"!GO_BIN!\go.exe" build -ldflags="-s -w" -o ".deploy\ginchat_server" .
if !ERRORLEVEL! NEQ 0 (
    echo [FAIL] Go build failed - is Go installed?
    pause
    exit /b 1
)
echo [OK] Go build done.

echo.
echo [2/3] Building frontend (Vue Vite)...
pushd frontend
if not exist "node_modules" (
    echo   Installing npm packages...
    call npm install
    if !ERRORLEVEL! NEQ 0 (
        echo [FAIL] npm install failed
        popd
        pause
        exit /b 1
    )
)
call npm run build
if !ERRORLEVEL! NEQ 0 (
    echo [FAIL] npm build failed
    popd
    pause
    exit /b 1
)
popd
echo [OK] Frontend build done.

echo.
echo [3/3] Collecting artifacts...

if exist ".deploy\frontend" (
    rmdir /s /q ".deploy\frontend"
)
mkdir ".deploy\frontend\dist"

xcopy /e /y /q "frontend\dist\*" ".deploy\frontend\dist\" >nul

echo [OK] Artifacts collected.

echo.
echo ============================================
echo   Build Complete!
echo ============================================
echo.
echo   Output files:
echo     .deploy\ginchat_server
echo     .deploy\frontend\dist\
echo.
echo   Next: upload these to BT Panel
echo     /www/wwwroot/ginchat-server/
echo     releases\YYYYMMDD-HHMMSS-githash\
echo.
echo ============================================

pause
endlocal
