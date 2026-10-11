# OpenWrt Mobile Controller

## System Overview

The OpenWrt Mobile Controller is a lightweight backend service written in Go that
lets administrators manage OpenWrt router settings from a mobile-optimized
Progressive Web App (PWA). It exposes a JSON HTTP API and translates every request
into `uci` commands executed on the router over SSH (using `golang.org/x/crypto/ssh`).

Two capabilities are supported:

1. **Static DHCP allocations** - create, list and delete `config host` sections in `/etc/config/dhcp`.
2. **MAC address whitelisting** - manage the wireless allow list (`macfilter='allow'` plus `maclist`) in `/etc/config/wireless`.

Architecture:

```
Mobile PWA (static, Tailwind via CDN)
        |
        v
Go HTTP service (net/http)  --- SSH (golang.org/x/crypto/ssh) --->  OpenWrt router (uci)
```

## Understanding the project (for beginners)

New to Go or to OpenWrt? Read these documents, in this order:

| Document | What it explains |
| :--- | :--- |
| [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) | How the pieces fit together and what happens on every click. |
| [docs/GO_FOR_PYTHON_DEVELOPERS.md](docs/GO_FOR_PYTHON_DEVELOPERS.md) | The Go concepts used here, explained for Python developers. |
| [docs/CODE_WALKTHROUGH.md](docs/CODE_WALKTHROUGH.md) | A guided tour of every Go file, plus how to add a new endpoint. |
| [docs/GLOSSARY.md](docs/GLOSSARY.md) | Plain language explanations of the networking words (UCI, SSH, DHCP, MAC, ...). |

The Go source files are also commented with this same idea in mind: each file starts
with a short explanation, and the trickier functions have notes that compare them to
Python when that helps.


## Prerequisites

Local development:

* Go 1.26.8 or newer.
* Network access from the Go process to the router SSH port (default 22).

Docker execution:

* Docker (or Podman) with the BuildKit/build capabilities enabled.
* Network access from the container to the router SSH port.

## Configuration

The service is fully configured through environment variables. No credentials are
baked into the image.

| Variable | Required | Default | Description |
| :--- | :--- | :--- | :--- |
| `ENV_FILE_PATH` | No | `.env` | Optional path to a `.env` file whose values are loaded at start up. |
| `LISTEN_ADDRESS` | No | `:8080` | Address the HTTP server binds to. |
| `STATIC_DIRECTORY` | No | `/app/static` | Directory that contains the PWA. |
| `API_BEARER_TOKEN` | No | *(empty)* | When set, `/api/*` requires `Authorization: Bearer <token>`. |
| `ROUTER_ADDRESS` | **Yes** | - | Router SSH `host` or `host:port` (port 22 assumed). |
| `ROUTER_USERNAME` | **Yes** | - | Router SSH username. |
| `ROUTER_PASSWORD` | No* | - | Router SSH password. |
| `ROUTER_PRIVATE_KEY_PATH` | No* | - | Path to a private key inside the container. |
| `ROUTER_PRIVATE_KEY` | No* | - | Inline private key content. |
| `ROUTER_PRIVATE_KEY_PASSPHRASE` | No | - | Passphrase for the private key. |
| `ROUTER_HOST_KEY_FINGERPRINT` | No | - | Expected host key fingerprint (SHA256). When empty, host key verification is disabled and a warning is logged. |
| `ROUTER_SSH_TIMEOUT_SECONDS` | No | `10` | SSH connection timeout. |
| `DNSMASQ_SERVICE_NAME` | No | `dnsmasq` | Service restarted after DHCP changes. |
| `ROUTER_WIRELESS_SECTIONS` | No | *(auto)* | Comma-separated `wifi-iface` sections to manage. When empty, every `wifi-iface` section in `ap` mode is used. |

\* At least one authentication method (`ROUTER_PASSWORD` or a private key) is required.

### Using a .env file (optional)

Instead of passing every setting on the command line you can keep them in a `.env`
file. Copy the example and edit it:

```bash
cp .env.example .env
```

The program loads `.env` automatically at start up; set `ENV_FILE_PATH` to use a
different path. Real environment variables always win over the file, so a value
exported in the shell is never overwritten. A missing file is simply ignored.

```dotenv
ROUTER_ADDRESS=192.168.1.1
ROUTER_USERNAME=root
ROUTER_PASSWORD=change-me
```

With Docker or Podman, either let the runtime inject the file:

```bash
docker run --env-file .env ...
```

or mount the file and let the program read it (useful when the same file must also
work for a local `go run .`):

```bash
docker run -v "$PWD/.env:/run/config/.env:ro" -e ENV_FILE_PATH=/run/config/.env ...
```

> The `.env` file is ignored by Git and by the Docker build context (see
> `.gitignore` and `.dockerignore`), so credentials never end up in the repository
> or in the image.

### HTTP API

| Method | Path | Description |
| :--- | :--- | :--- |
| `GET` | `/api/health` | Service health. |
| `GET` | `/api/static-dhcp-hosts` | List static DHCP reservations. |
| `POST` | `/api/static-dhcp-hosts` | Create a reservation (`name`, `macAddress`, `ipAddress`). |
| `DELETE` | `/api/static-dhcp-hosts/{macAddress}` | Delete a reservation by MAC address. |
| `GET` | `/api/mac-whitelist` | List allowed MAC addresses. |
| `POST` | `/api/mac-whitelist` | Allow a MAC address (`macAddress`). |
| `DELETE` | `/api/mac-whitelist/{macAddress}` | Revoke a MAC address. |

> Note: adding the first MAC address switches the target wireless interface into
> allow (whitelist) mode, which blocks every client that is not listed.

## Frontend (Progressive Web App)

The `static/` directory contains a mobile-first Progressive Web App built with the
Tailwind CSS Play CDN and vanilla JavaScript, with no build step:

| File | Purpose |
| :--- | :--- |
| `static/index.html` | Responsive app shell: bottom navigation and cards on mobile, sidebar and tables on desktop. |
| `static/app.js` | API client, view rendering and event wiring. |
| `static/theme-init.js` | Applies the stored color theme before the first paint. |
| `static/tailwind-config.js` | Enables class based dark mode for the Play CDN. |
| `static/service-worker.js` | Caches the app shell; `/api/*` requests always use the network. |
| `static/manifest.json` | Web app manifest. |
| `static/icon.svg` | Application icon. |

The Settings view stores the optional bearer token and an alternative API base URL
in the browser `localStorage`; the dark mode preference is stored there as well.
MAC address fields accept the compact form (`AABBCCDDEEFF`); colons are inserted
automatically and the API always stores the canonical `AA:BB:CC:DD:EE:FF` form.
Installing the PWA requires HTTPS or `localhost`.


## Deployment Instructions

### Build the image

```bash
docker build -t openwrt-controller:local .
```

### Run the container

```bash
docker run --rm -p 8080:8080 \
  -e ROUTER_ADDRESS=192.168.1.1 \
  -e ROUTER_USERNAME=root \
  -e ROUTER_PASSWORD=your-password \
  -e API_BEARER_TOKEN=your-api-token \
  openwrt-controller:local
```

Then open `http://localhost:8080/`.

### Docker Compose (alternative)

The provided `docker-compose.yml` builds and runs the service. Configuration is
read from the shell environment, so export the required variables first:

```bash
export ROUTER_ADDRESS=192.168.1.1
export ROUTER_USERNAME=root
export ROUTER_PASSWORD=your-password
# optional: API_BEARER_TOKEN, ROUTER_HOST_KEY_FINGERPRINT, LISTEN_PORT, ...

docker compose up -d --build
```

`ROUTER_ADDRESS` and `ROUTER_USERNAME` are required; Compose aborts with a clear
message when they are missing. Stop the stack with `docker compose down`.


### Run locally without Docker

```bash
ROUTER_ADDRESS=192.168.1.1 \
ROUTER_USERNAME=root \
ROUTER_PASSWORD=your-password \
STATIC_DIRECTORY=./static \
go run .
```

### Tests

```bash
go test ./...
```
