module.exports = {
  apps: [
    {
      name: "map-data-gateway",
      cwd: __dirname,
      script: "./map-data-gateway",
      interpreter: "none",
      env: {
        GATEWAY_HOST: "127.0.0.1",
        GATEWAY_PORT: "3000",
        GATEWAY_AUTH_TOKEN: "new-token123",
        GATEWAY_JAVA_BASE_URL: "http://127.0.0.1:8080",
        GATEWAY_JAVA_BIN: "java",
        GATEWAY_JAVA_JAR: "../build/libs/map-data-fetcher-0.0.1-SNAPSHOT.jar",
        GATEWAY_JAVA_HEADLESS: "true",
        GATEWAY_JAVA_IDLE_TIMEOUT: "5m",
        GATEWAY_JAVA_STARTUP_TIMEOUT: "90s",
        GATEWAY_JAVA_SHUTDOWN_TIMEOUT: "15s",
        GATEWAY_SERVICE_NAME: "map-data-gateway",
        GATEWAY_VERSION: "0.1.0",
        GATEWAY_SHUTDOWN_TIMEOUT: "5s"
      }
    }
  ]
};
