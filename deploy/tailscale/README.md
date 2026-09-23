# Tailscale MCP deployment

The Go process listens privately on `127.0.0.1:3000/mcp`. Tailscale exposes
that handler at `/naver/mcp` on dedicated HTTPS port `10000` so it can coexist
with other services on the same node. Serve and Funnel visibility applies to
an entire HTTPS port, so do not place this route on a port used by another
private service.

## Start the Go server

Build and install the two runtime artifacts and the user-service unit:

```bash
bash tests/build-baseline
install -Dm755 build/baseline/map-data-gateway \
  "$HOME/.local/libexec/map-data-fetcher/map-data-gateway"
install -Dm644 build/libs/map-data-fetcher-0.0.1-SNAPSHOT.jar \
  "$HOME/.local/share/map-data-fetcher/map-data-fetcher.jar"
install -Dm644 deploy/systemd/map-data-fetcher.service \
  "$HOME/.config/systemd/user/map-data-fetcher.service"
```

Create the private environment file referenced by the unit and put only the
bearer token in it:

```bash
install -d -m 700 "$HOME/.config/map-data-fetcher"
touch "$HOME/.config/map-data-fetcher/gateway.env"
chmod 600 "$HOME/.config/map-data-fetcher/gateway.env"
${EDITOR:-nano} "$HOME/.config/map-data-fetcher/gateway.env"
```

Add one line in the editor: `GATEWAY_AUTH_TOKEN=replace-with-a-secret`.

Then load and start the service. Lingering lets the user service survive logout
and start during boot without an interactive login:

```bash
systemctl --user daemon-reload
systemctl --user enable --now map-data-fetcher.service
loginctl enable-linger "$USER"
```

Check it with `systemctl --user status map-data-fetcher.service` and
`journalctl --user -u map-data-fetcher.service`. After installing rebuilt
artifacts, run `systemctl --user restart map-data-fetcher.service`.

## Configure Tailscale

For tailnet-only access:

```bash
TAILSCALE_MODE=serve deploy/tailscale/serve.sh
```

For a client outside the tailnet, use Funnel instead:

```bash
TAILSCALE_MODE=funnel deploy/tailscale/serve.sh
```

The script adds `/naver/mcp` on port `10000` without resetting existing
Tailscale routes. Override it with `TAILSCALE_HTTPS_PORT` only when the chosen
port has no unrelated routes. Do not use `tailscale serve reset` because other
services share this node.

Inspect the resulting route with:

```bash
tailscale serve status
```

The MCP client URL is the node's HTTPS hostname plus port `10000` and
`/naver/mcp`, for example
`https://node.example.ts.net:10000/naver/mcp`. It still needs the Go boundary
token:

```http
Authorization: Bearer <GATEWAY_AUTH_TOKEN>
```

## Verify the external endpoint

The normal integration script continues to test the local endpoint. Set
`EXTERNAL_MCP_URL` to additionally run MCP initialization against Tailscale:

```bash
EXTERNAL_MCP_URL='https://node.example.ts.net:10000/naver/mcp' bash tests/run-script
```

The check verifies HTTP 200 and a valid MCP initialization response.

## Remove the route

Remove the dedicated HTTPS listener using the same mode used to create it:

```bash
tailscale serve --https=10000 off
```

Use `tailscale funnel --https=10000 off` if Funnel was used. Visibility applies
to the whole port, so only use this command for the dedicated port.

Then stop the Go process:

```bash
systemctl --user disable --now map-data-fetcher.service
```
