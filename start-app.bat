@echo off
setlocal EnableDelayedExpansion

rem Define colors for Windows console
set "GREEN=[92m"
set "RED=[91m"
set "YELLOW=[93m"
set "RESET=[0m"

echo %GREEN%=== Child Malnutrition Prediction Web Application ===%RESET%
echo.

echo Choose execution mode:
echo   [1] Start Full Application (Go Frontend + Python Backend)
echo   [2] Start Go Frontend Only (Port 8080)
echo   [3] Docker Container Run
echo.

set /p choice="Enter choice [1, 2, or 3] (default 1): "
if "%choice%"=="" set choice=1

if "%choice%"=="1" (
    call start-all.bat
    exit /b 0
)

if "%choice%"=="2" (
    echo %GREEN%Starting Go Web Frontend on http://localhost:8080...%RESET%
    start http://localhost:8080
    go run main.go
    exit /b 0
)

rem Docker execution
set IMAGE_NAME=predicting-malnutrition
set CONTAINER_NAME=malnutrition-app
set PORT=8080

where docker >nul 2>nul
if %ERRORLEVEL% neq 0 (
    echo %RED%Docker is not installed. Please install Docker Desktop first or run Option 1.%RESET%
    pause
    exit /b 1
)

echo %YELLOW%Checking for existing Docker image...%RESET%
docker image inspect %IMAGE_NAME% >nul 2>nul
if %ERRORLEVEL% neq 0 (
    echo %YELLOW%Image '%IMAGE_NAME%' not found. Building it now...%RESET%
    docker build -t %IMAGE_NAME% .
    if %ERRORLEVEL% neq 0 (
        echo %RED%Failed to build Docker image.%RESET%
        exit /b 1
    )
)

docker ps -a --format "{{.Names}}" | findstr /b /e %CONTAINER_NAME% >nul
if %ERRORLEVEL% equ 0 (
    docker ps --format "{{.Names}}" | findstr /b /e %CONTAINER_NAME% >nul
    if %ERRORLEVEL% equ 0 (
        echo %GREEN%Container '%CONTAINER_NAME%' is already running.%RESET%
    ) else (
        echo %YELLOW%Starting container '%CONTAINER_NAME%'...%RESET%
        docker start %CONTAINER_NAME% >nul 2>nul
    )
) else (
    echo %YELLOW%Creating and running container '%CONTAINER_NAME%'...%RESET%
    docker run -d --name %CONTAINER_NAME% -p %PORT%:%PORT% %IMAGE_NAME%
)

echo.
echo %GREEN%Application is now running at: http://localhost:%PORT%%RESET%
start http://localhost:%PORT%

endlocal
