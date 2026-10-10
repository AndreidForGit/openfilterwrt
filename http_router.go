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

type bearerTokenHandler struct {
	nextHandler http.Handler
	bearerToken string
}

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

type requestLoggingHandler struct {
	nextHandler http.Handler
	logger      *log.Logger
}

func (handler requestLoggingHandler) ServeHTTP(responseWriter http.ResponseWriter, request *http.Request) {
	startTime := time.Now()
	handler.nextHandler.ServeHTTP(responseWriter, request)
	elapsedDuration := time.Since(startTime)

	handler.logger.Printf("%s %s from %s in %s", request.Method, request.URL.Path, request.RemoteAddr, elapsedDuration)
}

func extractBearerToken(authorizationHeader string) string {
	if strings.HasPrefix(authorizationHeader, bearerTokenPrefix) == false {
		return ""
	}

	tokenValue := authorizationHeader[len(bearerTokenPrefix):]
	trimmedTokenValue := strings.TrimSpace(tokenValue)
	return trimmedTokenValue
}
