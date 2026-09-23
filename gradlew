#!/bin/sh
set -e

APP_HOME=$(cd "$(dirname "$0")"; pwd -P)
WRAPPER_JAR="$APP_HOME/gradle/wrapper/gradle-wrapper.jar"

if [ ! -f "$WRAPPER_JAR" ]; then
  echo "Missing tracked Gradle wrapper JAR: $WRAPPER_JAR" >&2
  echo "Restore gradle/wrapper/gradle-wrapper.jar from git before running Gradle." >&2
  exit 1
fi

exec java -Dorg.gradle.appname=gradlew -classpath "$WRAPPER_JAR" org.gradle.wrapper.GradleWrapperMain "$@"
