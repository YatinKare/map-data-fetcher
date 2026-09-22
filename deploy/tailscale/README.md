# Tailscale MCP deployment

The Go process listens privately on `127.0.0.1:3000/mcp`. Tailscale exposes
that handler at `/naver/mcp` so it can coexist with other MCP services on the
same node.

## Start the Go server

Set the bearer token outside the repository, then start the PM2 application:

```bash
export GATEWAY_AUTH_TOKEN='replace-with-a-secret'
pm2 start gateway/ecosystem.config.cjs
```

`gateway/ecosystem.config.cjs` reads `GATEWAY_AUTH_TOKEN` from the shell; the
secret is not stored in the repository.

## Configure Tailscale

For tailnet-only access:

```bash
TAILSCALE_MODE=serve deploy/tailscale/serve.sh
```

For a client outside the tailnet, use Funnel instead:

```bash
TAILSCALE_MODE=funnel deploy/tailscale/serve.sh
```

The script adds `/naver/mcp` without resetting existing Tailscale routes. It
must not be used with `tailscale serve reset` because other MCP namespaces may
share this node.

Inspect the resulting route with:

```bash
tailscale serve status
```

The MCP client URL is the node's HTTPS hostname plus `/naver/mcp`. It still
needs the Go boundary token:

```http
Authorization: Bearer <GATEWAY_AUTH_TOKEN>
```

## Verify the external endpoint

The normal integration script continues to test the local endpoint. Set
`EXTERNAL_MCP_URL` to additionally run MCP initialization against Tailscale:

```bash
EXTERNAL_MCP_URL='https://node.example.ts.net/naver/mcp' bash tests/run-script
```

The check verifies HTTP 200 and a valid MCP initialization response.

## Remove the route

Remove only the `/naver/mcp` mount using the same mode and path used to create
it:

```bash
TAILSCALE_MODE=serve tailscale serve --set-path=/naver/mcp off
```

Use `tailscale funnel --set-path=/naver/mcp off` if Funnel was used.

Then stop the Go process:

```bash
pm2 delete map-data-gateway
```
