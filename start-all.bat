@echo off
setlocal EnableDelayedExpansion

title Malnutrition Prediction System Launcher
echo ============================================================
echo   Child Malnutrition Prediction System Launcher
echo ============================================================
echo.

rem Check for Python backend directory
if not exist "..\Malnutrition-Prediction-python-service" (
    echo [ERROR] Backend service directory ..\Malnutrition-Prediction-python-service not found.
    pause
    exit /b 1
)

rem Check if Port 8000 (Python Backend) is already listening
netstat -ano | findstr /R /C:":8000 .*LISTENING" >nul 2>nul
if %ERRORLEVEL% equ 0 (
    echo [OK] Python Inference Service is already running on port 8000.
) else (
    echo [1/2] Launching Python FastAPI Inference Service (Port 8000)...
    start "Python ML Backend (Port 8000)" cmd /k "cd /d ..\Malnutrition-Prediction-python-service && if exist venv\Scripts\activate.bat (call venv\Scripts\activate.bat) && python -m uvicorn app.main:app --host 127.0.0.1 --port 8000"
    echo Waiting 2 seconds for Python backend to initialize...
    timeout /t 2 /nobreak >nul
)

rem Check if Port 8080 (Go Frontend) is already listening
netstat -ano | findstr /R /C:":8080 .*LISTENING" >nul 2>nul
if %ERRORLEVEL% equ 0 (
    echo [OK] Go Web Frontend is already running on port 8080.
    echo.
    echo Opening browser at http://localhost:8080 ...
    start http://localhost:8080
    echo ============================================================
    echo   Application is active and ready in your browser!
    echo ============================================================
    timeout /t 5 >nul
    exit /b 0
)

echo [2/2] Launching browser at http://localhost:8080 ...
start http://localhost:8080

echo Starting Go Web Server on http://localhost:8080 ...
echo (Keep this window open while using the application)
echo.
go run main.go
