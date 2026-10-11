# Architecture

This document explains, in plain language, how the OpenWrt Mobile Controller is
built and what happens behind the scenes. It is written for someone who is new to
Go and to networking.

## What the project does

The project is a small web application that lets you manage two settings on an
OpenWrt router from a phone or a computer:

1. **Static DHCP reservations** - always give the same IP address to a device.
2. **Wireless MAC address allow list** - decide which devices may join the Wi-Fi.

The web page (the "frontend") is a Progressive Web App (PWA). The program that
does the real work (the "backend") is written in Go. The backend does not use a
special protocol to talk to the router: it opens an **SSH** connection and runs
the same `uci` commands you would type by hand.

## The big picture

```
+----------------------+       HTTP + JSON        +-----------------------------+
|  PWA in the browser  |  --------------------->  |  Go backend (this project)  |
|  static/index.html   |  <---------------------  |  net/http + JSON API        |
+----------------------+       JSON responses     +--------------+--------------+
                                                                 |
                                                                 | SSH (port 22)
                                                                 | golang.org/x/crypto/ssh
                                                                 v
                                                   +-----------------------------+
                                                   |  OpenWrt router             |
                                                   |  uci ...                    |
                                                   |  reads/writes /etc/config/* |
                                                   +-----------------------------+
```

## The moving parts

| Part | Where it lives | What it is |
| :--- | :--- | :--- |
| PWA | `static/` | HTML, CSS (Tailwind via CDN) and plain JavaScript. It is a normal web page that can be installed like an app. |
| HTTP API | `http_router.go`, `http_handlers.go` | The backend routes and the JSON endpoints such as `/api/mac-whitelist`. |
| Services | `static_dhcp_service.go`, `mac_whitelist_service.go` | The business logic for the two features. |
| UCI client | `uci_client.go` | Runs `uci` commands and parses the text they print. |
| SSH connection | `router_connection.go` | Opens one SSH connection to the router and reuses it. |
| Configuration | `configuration.go` | Reads the settings from environment variables. |
| Entry point | `main.go` | Builds everything and starts the HTTP server. |
| Validation | `validation.go` | Checks MAC and IP addresses before touching the router. |

## What happens when you open the app

1. The browser downloads `index.html`, `app.js` and the other files from the
   `static/` directory.
2. `app.js` runs. It asks `GET /api/health`. If it works, the green dot appears.
3. It then asks `GET /api/static-dhcp-hosts` and `GET /api/mac-whitelist` to fill
   the two lists.
4. Every 30 seconds it asks `/api/health` again to keep the indicator up to date.

## What happens when you add a DHCP reservation

1. `app.js` sends `POST /api/static-dhcp-hosts` with the hostname, the MAC address
   and the IP address.
2. `handleCreateStaticDHCPHost` normalizes the MAC address and calls
   `StaticDHCPService.CreateStaticDHCPHost`.
3. The service runs `uci add dhcp host`, sets the `name`, `mac` and `ip` options,
   then `uci commit dhcp` and restarts dnsmasq.
4. Because the two features usually go together, the handler then calls
   `MACWhitelistService.AddMACWhitelistEntry` with the same MAC address.
5. If that second step fails, the reservation is still kept and the response has a
   `warning` field, so the page can tell the user what went wrong.

Deleting a reservation does not remove the MAC address from the allow list. That is
safer, because the device keeps working until an administrator removes it on
purpose.

## What happens when you edit a reservation

Editing a reservation uses the same dialog as adding one: the MAC address field is
filled in and the Wi-Fi allow list warning is shown.

1. `app.js` sends `PUT /api/static-dhcp-hosts/<current MAC address>` with the new
   values.
2. `handleUpdateStaticDHCPHost` normalizes both MAC addresses and calls
   `StaticDHCPService.UpdateStaticDHCPHost`.
3. The service finds the section with the current MAC address. If the MAC address
   changed it recreates the section (`uci delete` then `uci add`), because UCI has
   no way to rename a section; otherwise it only writes the new `name` and `ip`.
4. It validates the change and refuses a MAC address that another reservation
   already uses (HTTP `409 Conflict`).
5. Back in the handler, the allow list is synchronized: the new MAC address is
   allowed and, if the MAC address changed, the old one is removed unless another
   reservation still uses it.

## What happens when you click "Allow address"

This is the most interesting flow, because it shows how one click travels all the
way to the router.

1. `app.js` sends `POST /api/mac-whitelist` with a body like
   `{"macAddress":"AA:BB:CC:DD:EE:FF"}`. The browser adds the JSON content type
   and, if a token was saved in the Settings view, an `Authorization` header.
2. The router (`http_router.go`) sends the request to
   `handleAddMACWhitelistEntry`.
3. That handler calls `MACWhitelistService.AddMACWhitelistEntry`.
4. The service first reads the current wireless configuration by running
   `uci show wireless` over SSH, and works out which `wifi-iface` sections are
   access points.
5. If every access point is already in allow mode and already lists that MAC
   address, there is nothing to do and the service stops here. No commit and no
   reload happen, so connected clients are not disturbed.
6. Otherwise, for each access point it turns the MAC filter on with
   `uci set ... macfilter='allow'` and adds the MAC address unless it is already
   listed (this avoids duplicates).
7. It saves the change with `uci commit wireless`.
8. It applies the change with `wifi reload`.
9. The handler answers `201 Created`, and the page refreshes its list.

The allow list page also shows the device name of every MAC address that has a
static DHCP reservation. The browser joins the two lists, so the backend does not
need to know about that relationship.

The commands that actually run on the router look like this:

```
uci show wireless
uci set wireless.default_radio0.macfilter='allow'
uci add_list wireless.default_radio0.maclist='AA:BB:CC:DD:EE:FF'
uci commit wireless
wifi reload
```

## Why the SSH reader is careful

`uci show wireless` prints a list option as several quoted values on **one line**:

```
wireless.default_radio0.maclist='AA:BB:CC:DD:EE:01' 'AA:BB:CC:DD:EE:02'
```

If the program treated that whole line as one value, the list would look broken.
That is why `parseUCIValueTokens` splits it into separate values; this is the kind
of detail that is easy to miss when you only test against fake data.

## Security notes

- The backend never stores router credentials; they come from environment
  variables at start up.
- `ROUTER_HOST_KEY_FINGERPRINT` pins the router identity. Without it the program
  prints a warning and accepts any host key (fine for a lab, risky in real life).
- `API_BEARER_TOKEN` protects the `/api/` routes. The static files stay public so
  the page can load and ask for the token.
- The container runs as a non-root user and with a read-only filesystem.

## How to read the code

Start with `main.go` to see the order in which everything is created, then read
`configuration.go` (the settings) and `router_connection.go` (how a command
reaches the router). After that, pick one feature, for example
`mac_whitelist_service.go`, and follow it up into `http_handlers.go` (the API) and
`uci_client.go` (the commands).

