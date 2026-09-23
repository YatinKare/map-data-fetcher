## map-data-fetcher

This project runs a lightweight Go Streamable HTTP MCP server in front of a
lazy Java/Selenium worker for Naver Maps searches.

## Runtime architecture

- Go MCP server: `127.0.0.1:3000`
- Local MCP path: `/mcp`
- Tailscale MCP URL: `https://<node>.ts.net:10000/naver/mcp`
- Private Java worker: `127.0.0.1:8080`
- Java startup: lazy, on the first Naver tool call
- Java shutdown: after the configured idle timeout
- Authentication: bearer token by default; set `GATEWAY_AUTH_MODE=none` for a
  public no-auth deployment

MCP is the only external application interface. The Java HTTP endpoints remain
private worker endpoints used by the Go process; they are not public API
documentation or a second external interface.

## MCP tools

The server exposes two read-only tools:

- `naver_map_search`: keyword search using `query` and optional `page`.
- `naver_map_coordinate_search`: coordinate search using `query`, `x`
  (longitude), `y` (latitude), and optional `page`.

Both tools return the raw JSON extracted by the existing Java worker.

## Local verification

Build the Java worker and Go server, then run the integration script:

```bash
bash tests/build-baseline
bash tests/run-script
```

The script verifies the configured authentication mode, MCP initialization, tool
discovery, keyword search, coordinate search, lazy Java startup, idle shutdown,
and worker restart. Each search must return at least 10 results by default.

## Tailscale deployment

The Go server stays on loopback while Tailscale publishes the external
`/naver/mcp` path. See [deploy/tailscale/README.md](deploy/tailscale/README.md)
for Serve/Funnel setup, bearer-token configuration, verification, and cleanup.

For an external endpoint check, provide `EXTERNAL_MCP_URL`:

```bash
EXTERNAL_MCP_URL='https://node.example.ts.net:10000/naver/mcp' bash tests/run-script
```

## Deployment configuration

`deploy/systemd/map-data-fetcher.service` keeps the Go process running as a
user service. Set `GATEWAY_AUTH_MODE=required` and store
`GATEWAY_AUTH_TOKEN` in the service's private environment file for bearer
authentication. Set `GATEWAY_AUTH_MODE=none` for a public no-auth deployment;
add rate limiting and other abuse protections before exposing it publicly.

The Java application still exposes its private worker routes under
`/api/naver-map/*` and `/healthz` on port `8080`. Do not expose that port
through Tailscale or another public proxy.
