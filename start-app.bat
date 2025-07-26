@echo off
setlocal EnableDelayedExpansion

rem Define colors for Windows console
set "GREEN=[92m"
set "RED=[91m"
set "YELLOW=[93m"
set "RESET=[0m"

rem Configuration
set IMAGE_NAME=predicting-malnutrition
set CONTAINER_NAME=malnutrition-app
set PORT=8080

echo %GREEN%=== Malnutrition Prediction App Docker Automation ===%RESET%
echo.

rem Check if Docker is installed
where docker >nul 2>nul
if %ERRORLEVEL% neq 0 (
    echo %RED%Docker is not installed. Please install Docker Desktop first.%RESET%
    exit /b 1
)

rem Suppress standard Docker output
echo %YELLOW%Checking for existing Docker image...%RESET%
docker image inspect %IMAGE_NAME% >nul 2>nul
if %ERRORLEVEL% neq 0 (
    echo %YELLOW%Image '%IMAGE_NAME%' not found. Building it now...%RESET%
    
    rem Build the Docker image (hide standard output, show only errors)
    docker build -t %IMAGE_NAME% . >nul
    if %ERRORLEVEL% neq 0 (
        echo %RED%Failed to build the image. Please check your Dockerfile and try again.%RESET%
        exit /b 1
    ) else (
        echo %GREEN%Image built successfully!%RESET%
    )
) else (
    echo %GREEN%Image '%IMAGE_NAME%' found.%RESET%
)

rem Check if container exists (running or stopped)
echo %YELLOW%Checking for existing container...%RESET%
docker ps -a --format "{{.Names}}" | findstr /b /e %CONTAINER_NAME% >nul
if %ERRORLEVEL% equ 0 (
    rem Container exists, check if it's running
    docker ps --format "{{.Names}}" | findstr /b /e %CONTAINER_NAME% >nul
    if %ERRORLEVEL% equ 0 (
        echo %GREEN%Container '%CONTAINER_NAME%' is already running.%RESET%
    ) else (
        echo %YELLOW%Container '%CONTAINER_NAME%' exists but is not running. Starting it now...%RESET%
        
        rem Start the existing container (hide output)
        docker start %CONTAINER_NAME% >nul 2>nul
        if %ERRORLEVEL% neq 0 (
            echo %RED%Failed to start the container.%RESET%
            exit /b 1
        ) else (
            echo %GREEN%Container started successfully!%RESET%
        )
    )
) else (
    echo %YELLOW%No container named '%CONTAINER_NAME%' found. Creating a new one...%RESET%
    
    rem Create and run a new container (hide output)
    docker run -d --name %CONTAINER_NAME% -p %PORT%:%PORT% %IMAGE_NAME% >nul 2>nul
    if %ERRORLEVEL% neq 0 (
        echo %RED%Failed to create and start the container.%RESET%
        exit /b 1
    ) else (
        echo %GREEN%Container created and started successfully!%RESET%
    )
)

echo.
echo %GREEN%----------------------------------------%RESET%
echo %GREEN%Application is now running!%RESET%
echo %GREEN%Access your application at: http://localhost:%PORT%%RESET%
echo %GREEN%----------------------------------------%RESET%

endlocal