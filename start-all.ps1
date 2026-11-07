# Multi-Process Automation Script
Write-Host "============================================================" -ForegroundColor Cyan
Write-Host "  Child Malnutrition Prediction System Launcher" -ForegroundColor Cyan
Write-Host "============================================================" -ForegroundColor Cyan
Write-Host ""

$pythonDir = Resolve-Path "..\Malnutrition-Prediction-python-service"
if (-not (Test-Path $pythonDir)) {
    Write-Host "[ERROR] Backend service directory $pythonDir not found." -ForegroundColor Red
    exit 1
}

# Check if port 8000 is already active
$port8000Active = Get-NetTCPConnection -LocalPort 8000 -State Listen -ErrorAction SilentlyContinue
if ($port8000Active) {
    Write-Host "[OK] Python Inference Service is already running on port 8000." -ForegroundColor Green
} else {
    Write-Host "[1/2] Launching Python FastAPI Inference Service (Port 8000)..." -ForegroundColor Yellow
    $pyVenv = Join-Path $pythonDir "venv\Scripts\python.exe"
    if (Test-Path $pyVenv) {
        Start-Process -FilePath "cmd.exe" -ArgumentList "/k cd /d `"$pythonDir`" && `"$pyVenv`" -m uvicorn app.main:app --host 127.0.0.1 --port 8000"
    } else {
        Start-Process -FilePath "cmd.exe" -ArgumentList "/k cd /d `"$pythonDir`" && python -m uvicorn app.main:app --host 127.0.0.1 --port 8000"
    }
    Start-Sleep -Seconds 2
}

# Check if port 8080 is already active
$port8080Active = Get-NetTCPConnection -LocalPort 8080 -State Listen -ErrorAction SilentlyContinue
if ($port8080Active) {
    Write-Host "[OK] Go Frontend is already running on port 8080." -ForegroundColor Green
    Write-Host "Opening browser at http://localhost:8080..." -ForegroundColor Green
    Start-Process "http://localhost:8080"
    Write-Host "Application is active and ready in your browser!" -ForegroundColor Cyan
    exit 0
}

Write-Host "[2/2] Launching browser at http://localhost:8080..." -ForegroundColor Green
Start-Process "http://localhost:8080"

Write-Host "Starting Go Web Frontend on port 8080..." -ForegroundColor Green
Write-Host "(Keep this window open while using the application)" -ForegroundColor Gray
go run main.go
