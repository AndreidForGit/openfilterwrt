# Glossary

Plain language explanations of the words and acronyms used in this project.

## API

"Application Programming Interface". Here it means the JSON endpoints the web page
calls, for example `GET /api/static-dhcp-hosts`. Think of it as the menu of things
the backend knows how to do.

## Bearer token

A simple password sent in the HTTP header `Authorization: Bearer <token>`. When
`API_BEARER_TOKEN` is set, the backend refuses `/api/` requests that do not send
the right token. It protects the API without a full login system.

## DHCP

"Dynamic Host Configuration Protocol". It is how a device asks the network for an
IP address when it connects. Normally the router gives out free addresses in any
order.

## dnsmasq

The small program inside OpenWrt that acts as the DHCP server (and as a DNS
forwarder). After changing DHCP settings the backend restarts it so the new
settings are used.

## Environment variable

A named setting that lives outside the program, for example `ROUTER_ADDRESS`. In
Python you read them with `os.environ`. They are used here so that no password or
IP address has to be written in the source code.

## MAC address

A unique identifier for a network card, written as six pairs of hexadecimal
digits, for example `AA:BB:CC:DD:EE:FF`. The web form accepts it with or without
the colons (`AABBCCDDEEFF`) and the program always stores the version with colons.

## Middleware

A piece of code that sits between the HTTP server and your handler. Here there are
two: one that logs every request and one that checks the bearer token. Each one
does its job and then calls the next one.

## OpenWrt

A Linux distribution for routers. It is configured with plain text files under
`/etc/config`.

## PWA (Progressive Web App)

A normal website that can be "installed" on a phone and can work a little bit like
a native app. The `manifest.json` file and the service worker are what make it a
PWA.

## Service worker

A small script (`static/service-worker.js`) that the browser runs in the
background. This one caches the page files so the app opens faster. It never caches
`/api/` responses, so the lists are always fresh.

## SSH

"Secure Shell". A way to log in to another machine and run commands on it. The
backend uses it to run `uci` on the router, using the Go library
`golang.org/x/crypto/ssh` instead of the system `ssh` program.

## Static DHCP reservation (static lease)

A rule that always gives the same IP address to one specific device, identified by
its MAC address. It is stored as a `config host` section in `/etc/config/dhcp`.

## UCI

"Unified Configuration Interface". The command line tool and the file format used
by OpenWrt for its settings. For example `uci show wireless` prints the wireless
configuration and `uci commit wireless` saves changes to disk.

## uci section and option

A UCI file is made of sections (for example a Wi-Fi interface) and each section has
options (for example `ssid` or `maclist`). A section can be named
(`default_radio0`) or anonymous (shown as `@wifi-iface[0]`).

## Wi-Fi MAC filter

A router feature that allows or blocks Wi-Fi clients based on their MAC address.
When it is set to `allow`, only the addresses in `maclist` may connect.
