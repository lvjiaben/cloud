@echo off
REM Purpose: Windows command wrapper for LinaPro's linactl tool.
REM It forwards all arguments to hack\tools\linactl.
setlocal
if not defined LINA_CORE_PORT set "LINA_CORE_PORT=6030"
if not defined LINA_VBEN_PORT set "LINA_VBEN_PORT=6031"
if not defined LINA_CUSTOMER_PORT set "LINA_CUSTOMER_PORT=6032"
pushd "%~dp0hack\tools\linactl" || exit /b 1
go run . %*
set EXIT_CODE=%ERRORLEVEL%
popd
exit /b %EXIT_CODE%
