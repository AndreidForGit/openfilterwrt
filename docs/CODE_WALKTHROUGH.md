# Code walkthrough

This is a guided tour of the Go files. Read them in this order.

## main.go - the entry point

`main` is what Go runs first. It does five small steps:

1. `LoadServerConfigurationFromEnvironment()` reads all the settings.
2. `NewRouterConnection(...)` and `NewUCIClient(...)` build the SSH pieces.
3. `NewStaticDHCPService(...)` and `NewMACWhitelistService(...)` build the two
   features.
4. `NewHTTPHandler(...)` wires the HTTP routes and the server starts in a
   goroutine with `go startHTTPServer(...)`.
5. The program waits on a channel for Ctrl+C or SIGTERM and then shuts down
   cleanly.

Notice how little logic lives here: it only creates objects and passes them to each
other. That pattern is often called "wiring".

## configuration.go - the settings

`LoadServerConfigurationFromEnvironment` reads environment variables and fills a
`ServerConfiguration` struct. Two variables are mandatory (`ROUTER_ADDRESS` and
`ROUTER_USERNAME`); if they are missing the program stops with a clear message.

Small helpers keep it readable: `readEnvironmentVariableOrDefault`,
`ensurePortPresent` (adds `:22` when there is no port) and
`splitCommaSeparatedValues`.

## router_connection.go - talking to the router

The important type is `RouterConnection`. It keeps one SSH client and reuses it.

- `ExecuteCommand` is the public method. If the connection has died, it opens a
  new one and tries once more.
- `ensureConnection` does the actual dialing, protected by a `sync.Mutex` so that
  two HTTP requests cannot dial at the same time.
- `buildAuthenticationMethods`, `parsePrivateKeySigner` and `buildHostKeyCallback`
  prepare the login: password, private key, and how to check the router identity.

`CommandExecutor` is the interface that hides all of this behind one method, which
is what makes the tests easy.

## uci_client.go - running and reading uci

`UCIClient` wraps the `CommandExecutor` with one method per uci command you need:
`ShowConfiguration`, `AddSection`, `SetOption`, `AddListValue`, `DeleteListValue`,
`DeleteSection`, `Commit`, `RestartService` and `ReloadWireless`.

Two details are worth your time:

- `runCommand` turns a non-zero exit status into a Go error, so callers do not
  have to look at `ExitStatus` themselves.
- `ParseUCIShowOutput` and `parseUCIValueTokens` read the text printed by
  `uci show`. A list option is printed as several quoted values on one line, and
  the parser splits them into separate values.

`BuildShellQuotedValue` adds the single quotes around a value so it is safe to pass
through the shell on the router.

## static_dhcp_service.go - reservations

`StaticDHCPService` exposes three methods used by the API:

- `ListStaticDHCPHosts` reads `/etc/config/dhcp` and returns the `host` sections.
- `CreateStaticDHCPHost` validates the input, refuses a duplicate MAC address, then
  runs `uci add dhcp host` and sets the `name`, `mac` and `ip` options.
- `DeleteStaticDHCPHost` finds the section with that MAC address and deletes it.

Both create and delete end with `uci commit dhcp` and a restart of dnsmasq. The
`mutationLock` makes sure two changes do not run at the same time.

## mac_whitelist_service.go - the allow list

`MACWhitelistService` is the same idea for the Wi-Fi allow list:

- `ListMACWhitelistEntries` reads `/etc/config/wireless` and returns every MAC in
  the `maclist` option of the access point sections.
- `AddMACWhitelistEntry` sets `macfilter` to `allow`, adds the address with
  `uci add_list` (skipping sections that already contain it) and reloads Wi-Fi.
- `RemoveMACWhitelistEntry` removes a single address with `uci del_list`.

Two helpers do the thinking: `targetSectionIdentifiers` picks which sections to
edit, and `sectionContainsMACAddress` avoids duplicates.

> Warning: the first address you add switches the Wi-Fi to allow-list mode, so any
> device that is not listed will be disconnected.

## validation.go - checking input

`ValidateMACAddress` and `ValidateIPv4Address` return a `ValidationError` when a
value is wrong. `NormalizeMACAddress` does double duty: it validates the address
and returns it in the canonical `AA:BB:CC:DD:EE:FF` form, so the rest of the code
never has to worry about upper case or missing colons.

`macAddressesEqual` compares two addresses ignoring case and separators.

## http_handlers.go - the JSON API

The `Application` struct holds the two services. Each handler is a method on it:

- It reads the request (a JSON body or a value from the URL).
- It calls a service method.
- It writes a JSON response with `writeJSONResponse`.

`serviceErrorStatusCode` decides the HTTP status: `400` for invalid input, `404`
when something is missing, `502` when the router itself failed.

## http_router.go - routes and middleware

`NewHTTPHandler` registers every route and wraps the router twice: logging on the
outside, bearer token check on the inside. It also serves the PWA files from the
`static` directory.

## The test files

- `test_helpers_test.go` defines `fakeCommandExecutor`, the fake router.
- `uci_client_test.go` checks the parser against real `uci show` output.
- `static_dhcp_service_test.go` and `mac_whitelist_service_test.go` check the exact
  commands that would be sent.
- `http_handlers_test.go` checks status codes and JSON using fake services.
- `validation_test.go` checks MAC and IP validation.

## Adding a new endpoint

1. Add the method to the right service (for example `MACWhitelistService`).
2. Add the matching method to the manager interface (for example
   `MACWhitelistManager`).
3. Write the handler in `http_handlers.go`.
4. Register the route in `http_router.go`.
5. Add tests with the fake executor or the fake manager.
6. Document it in `README.md`.

## Recap of one request

```
browser -> app.js -> POST /api/mac-whitelist
        -> requestLoggingHandler -> bearerTokenHandler -> ServeMux
        -> handleAddMACWhitelistEntry -> MACWhitelistService
        -> UCIClient -> RouterConnection -> SSH -> uci on the router
```


