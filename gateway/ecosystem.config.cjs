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
        GATEWAY_JAVA_BASE_URL: "http://127.0.0.1:8080",
        GATEWAY_SERVICE_NAME: "map-data-gateway",
        GATEWAY_VERSION: "0.1.0",
        GATEWAY_SHUTDOWN_TIMEOUT: "5s"
      }
    }
  ]
};
