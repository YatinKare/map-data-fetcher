# Install map-data-fetcher

Supported release platform: Linux x86-64.

Install the latest release for the current user with:

```bash
curl -fsSL https://github.com/YatinKare/map-data-fetcher/releases/latest/download/install.sh | bash
```

The installer creates a private bearer token on first install and enables the user systemd service
when a user systemd session is available. Existing configuration is preserved
on upgrades. Run the same command again to upgrade to the latest release.

The Java worker requires Java 17 or newer, Chrome or Chromium, and a matching
ChromeDriver. These are system dependencies and are not included in the
release archive. The MCP endpoint listens on `127.0.0.1:3000/mcp`.

To use the endpoint, configure an MCP client with that URL and the bearer token
stored in `~/.config/map-data-fetcher/gateway.env`. The service unit is
installed at `~/.config/systemd/user/map-data-fetcher.service`.
