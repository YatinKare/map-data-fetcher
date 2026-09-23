@echo off
setlocal

set APP_HOME=%~dp0
set WRAPPER_JAR=%APP_HOME%gradle\wrapper\gradle-wrapper.jar
set WRAPPER_MAIN=org.gradle.wrapper.GradleWrapperMain

if not exist "%WRAPPER_JAR%" (
  >&2 echo Missing tracked Gradle wrapper JAR: "%WRAPPER_JAR%"
  >&2 echo Restore gradle\wrapper\gradle-wrapper.jar from git before running Gradle.
  exit /b 1
)

java "-Dorg.gradle.appname=gradlew" -classpath "%WRAPPER_JAR%" %WRAPPER_MAIN% %*
if errorlevel 1 goto error
goto end

:error
echo Gradle wrapper execution failed.
exit /b 1

:end
endlocal
