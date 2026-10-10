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

- `naver_map_search` searches by required, nonblank `query` text. Use it when
  the request names a place, business, category, or location in prose. A
  location written in prose does not provide coordinates.
- `naver_map_coordinate_search` searches near a required numeric `longitude`
  and `latitude`, along with a required, nonblank `query`. Use it only when
  both coordinates are explicitly available. Coordinates use WGS84 decimal
  degrees: longitude is from -180 to 180 and latitude is from -90 to 90.

Both tools accept an optional `page`, defaulting to 1; valid pages are 1
through 5. Use a later page when the user asks for more results. If the requested
page cannot be found, the tool returns an error identifying the page and
suggesting an earlier page. A successfully retrieved page with no places returns
an empty array.

Both tools return an array of compact place results. Each result can include
`rank`, `id`, `name`, `category`, `road_address`, `coordinates`, `tel`,
`business_status`, `business_hours`, `break_time`, `last_order`,
`thumbnail_url`, `homepage`, `menu_info`, and `reservation_options`. Fields
without an upstream value are omitted. `category` and `reservation_options`
are arrays of strings. Coordinates contain `longitude` and `latitude` in WGS84
decimal degrees. Time ranges are returned as local `HH:mm–HH:mm` strings.
Phone numbers prefer the listed telephone number and fall back to Naver's
virtual telephone number when needed. MCP clients receive the result as both
structured data matching the published output schema and JSON text for
compatibility.

Review counts and Naver-specific distance, indoor, and subway fields are not
included. For example:

```json
[
  {
    "rank": 1,
    "id": "1479088801",
    "name": "태능감자탕 노원본점",
    "category": ["한식", "감자탕"],
    "road_address": "서울특별시 노원구 노해로83길 10-1 1층",
    "coordinates": {"longitude": 127.0644051, "latitude": 37.6558436},
    "tel": "0507-1362-8077",
    "business_status": "24시간 영업",
    "business_hours": "00:00–24:00",
    "thumbnail_url": "https://ldb-phinf.pstatic.net/example.jpg",
    "homepage": "https://taeneung.co.kr/",
    "menu_info": "태능식감자탕(중) 38,000 | 태능식감자탕(대) 45,000",
    "reservation_options": ["reservation"]
  }
]
```

MCP request logging is disabled by default. Toggle it on or off at runtime by
sending `SIGUSR1` to the gateway process. For the systemd user service, run:

```bash
systemctl --user kill -s SIGUSR1 map-data-fetcher.service
```

When enabled, request summaries go to standard error through Go's default
logger and are collected by systemd. View them with
`journalctl --user -u map-data-fetcher.service`.

## Local verification

Build the Java worker and Go server, then run the integration script:

```bash
bash tests/build-baseline
bash tests/run-script
```

The script verifies the configured authentication mode, MCP initialization, tool
discovery, keyword search, coordinate search, page 2 for both tools, lazy Java
startup, idle shutdown, and worker restart. Each search must return at least 10
results by default. The queries used for pagination checks must have at least
two pages of results. Page 2 must include place IDs absent from page 1. The worker
waits for Naver’s pagination click handler before selecting a page, then matches
the GraphQL response to that page’s result offset.

## Tailscale deployment

The Go server stays on loopback while Tailscale publishes the external
`/naver/mcp` path. See [deploy/tailscale/README.md](deploy/tailscale/README.md)
for Serve/Funnel setup, bearer-token configuration, verification, and cleanup.

## OpenAI Secure MCP Tunnel

Install and run OpenAI's [`tunnel-client`](https://github.com/openai/tunnel-client)
on a host that can reach the gateway, forwarding to `http://127.0.0.1:3000/mcp`;
create the tunnel in [OpenAI Platform tunnel settings](https://platform.openai.com/settings/organization/tunnels).
Keep the gateway bound to loopback and set `GATEWAY_AUTH_MODE=none` for this
local connection. To add the MCP server, use [ChatGPT on the web](https://chatgpt.com),
then choose **Add custom MCP server** and **Tunnel** for the connection; this
setup is not available in the ChatGPT desktop app. See OpenAI's [Secure MCP
Tunnel guide](https://developers.openai.com/api/docs/guides/secure-mcp-tunnels)
for tunnel-client setup details.

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

## Install a release

On Linux x86-64, install the latest release for the current user with:

```bash
curl -fsSL https://github.com/YatinKare/map-data-fetcher/releases/latest/download/install.sh | bash
```

The worker requires Java 17 or newer, Chrome or Chromium, and a matching
ChromeDriver.
See [the install guide](docs/INSTALL.md) for the endpoint and configuration
locations.

## Build a release

Maintainers can build a release bundle from a clean checkout of its exact Git
tag with `bash scripts/build-release.sh vX.Y.Z`. See
[the release guide](docs/RELEASE.md) for build requirements.
