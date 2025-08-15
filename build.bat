@echo off
REM ==============================
REM  Go Build Script for Windows
REM  Author: Sarwar Hossain
REM ==============================

REM Check if Go is installed
go version >nul 2>&1
if %errorlevel% neq 0 (
    echo [ERROR] Go is not installed or not in PATH.
    echo Download Go from https://go.dev/dl
    pause
    exit /b
)

REM Set variables
set "GOFILE=main.go"
set "OUTFILE=myprogram.exe"

REM Build
echo Building %OUTFILE% from %GOFILE%...
go build -ldflags="-s -w" -o %OUTFILE% %GOFILE%

REM Check build result
if %errorlevel% neq 0 (
    echo [ERROR] Build failed.
    pause
    exit /b
)

echo [SUCCESS] Build complete: %OUTFILE%
echo Run: %OUTFILE%
pause
