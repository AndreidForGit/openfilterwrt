// HTTP routing and middleware.
//
// NewHTTPHandler registers every route on an http.ServeMux and wraps it with two
// middleware layers: request logging on the outside and an optional bearer token
// check on the inside. A middleware is simply an http.Handler that does something
// before or after calling the next handler.
//
// The patterns used with HandleFunc, such as "GET /api/static-dhcp-hosts" and
// "DELETE /api/mac-whitelist/{macAddress}", are understood by the standard
// library since Go 1.22. The part in curly braces is read with
// request.PathValue("macAddress").
//
// Python equivalent: routing is like @app.route in Flask, and middleware is like
// a decorator applied to the whole application.

package main

import (
	"log"
	"net/http"
	"strings"
	"time"
)

const apiPathPrefix = "/api/"
const bearerTokenPrefix = "Bearer "

// NewHTTPHandler builds the full HTTP handler chain: request logging wraps the
// optional bearer token check, which in turn wraps the route multiplexer.
func NewHTTPHandler(application *Application, staticDirectory string, bearerToken string, logger *log.Logger) http.Handler {
	serveMux := http.NewServeMux()

	serveMux.HandleFunc("GET /api/health", application.handleHealthRequest)
	serveMux.HandleFunc("GET /api/static-dhcp-hosts", application.handleListStaticDHCPHosts)
	serveMux.HandleFunc("POST /api/static-dhcp-hosts", application.handleCreateStaticDHCPHost)
	serveMux.HandleFunc("DELETE /api/static-dhcp-hosts/{macAddress}", application.handleDeleteStaticDHCPHost)
	serveMux.HandleFunc("GET /api/mac-whitelist", application.handleListMACWhitelistEntries)
	serveMux.HandleFunc("POST /api/mac-whitelist", application.handleAddMACWhitelistEntry)
	serveMux.HandleFunc("DELETE /api/mac-whitelist/{macAddress}", application.handleRemoveMACWhitelistEntry)

	staticFileServer := http.FileServer(http.Dir(staticDirectory))
	serveMux.Handle("GET /", staticFileServer)

	bearerTokenProtectedHandler := bearerTokenHandler{
		nextHandler: serveMux,
		bearerToken: bearerToken,
	}
	requestLoggingProtectedHandler := requestLoggingHandler{
		nextHandler: bearerTokenProtectedHandler,
		logger:      logger,
	}

	return requestLoggingProtectedHandler
}

// bearerTokenHandler is middleware that protects the "/api/" routes with a bearer
// token. When no token is configured it simply forwards every request.
type bearerTokenHandler struct {
	nextHandler http.Handler
	bearerToken string
}

// ServeHTTP compares the incoming token with the configured one. Static files are
// not protected, so the PWA can always load and ask the user for the token.
func (handler bearerTokenHandler) ServeHTTP(responseWriter http.ResponseWriter, request *http.Request) {
	if handler.bearerToken == "" {
		handler.nextHandler.ServeHTTP(responseWriter, request)
		return
	}

	isAPIRequest := strings.HasPrefix(request.URL.Path, apiPathPrefix)
	if isAPIRequest == false {
		handler.nextHandler.ServeHTTP(responseWriter, request)
		return
	}

	providedToken := extractBearerToken(request.Header.Get("Authorization"))
	if providedToken != handler.bearerToken {
		writeErrorResponse(responseWriter, http.StatusUnauthorized, "invalid or missing bearer token")
		return
	}

	handler.nextHandler.ServeHTTP(responseWriter, request)
}

// requestLoggingHandler is middleware that prints one log line per HTTP request.
type requestLoggingHandler struct {
	nextHandler http.Handler
	logger      *log.Logger
}

// ServeHTTP measures how long the next handler took and writes the result to the
// logger.
func (handler requestLoggingHandler) ServeHTTP(responseWriter http.ResponseWriter, request *http.Request) {
	startTime := time.Now()
	handler.nextHandler.ServeHTTP(responseWriter, request)
	elapsedDuration := time.Since(startTime)

	handler.logger.Printf("%s %s from %s in %s", request.Method, request.URL.Path, request.RemoteAddr, elapsedDuration)
}

// extractBearerToken reads the token from an "Authorization: Bearer <token>"
// header. It returns an empty string when the header is missing or malformed.
func extractBearerToken(authorizationHeader string) string {
	if strings.HasPrefix(authorizationHeader, bearerTokenPrefix) == false {
		return ""
	}

	tokenValue := authorizationHeader[len(bearerTokenPrefix):]
	trimmedTokenValue := strings.TrimSpace(tokenValue)
	return trimmedTokenValue
}
