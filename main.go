package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

const (
	httpServerReadHeaderTimeout = 10 * time.Second
	httpServerShutdownTimeout   = 10 * time.Second
)

func main() {
	serverConfiguration, configurationError := LoadServerConfigurationFromEnvironment()
	if configurationError != nil {
		log.Fatalf("failed to load server configuration: %v", configurationError)
	}

	logger := log.New(os.Stdout, "", log.LstdFlags)

	if serverConfiguration.RouterHostKeyFingerprint == "" {
		logger.Printf("warning: ROUTER_HOST_KEY_FINGERPRINT is not set; SSH host key verification is disabled")
	}

	routerConnection := NewRouterConnection(serverConfiguration)
	uciClient := NewUCIClient(routerConnection)

	staticDHCPService := NewStaticDHCPService(uciClient, serverConfiguration.DNSMasqServiceName)
	macWhitelistService := NewMACWhitelistService(uciClient, serverConfiguration.RouterWirelessSections)

	application := &Application{
		StaticDHCPManager:   staticDHCPService,
		MACWhitelistManager: macWhitelistService,
	}

	httpRequestHandler := NewHTTPHandler(application, serverConfiguration.StaticDirectory, serverConfiguration.APIBearerToken, logger)

	httpServer := &http.Server{
		Addr:              serverConfiguration.ListenAddress,
		Handler:           httpRequestHandler,
		ReadHeaderTimeout: httpServerReadHeaderTimeout,
	}

	shutdownSignalChannel := make(chan os.Signal, 1)
	signal.Notify(shutdownSignalChannel, syscall.SIGINT, syscall.SIGTERM)

	serverErrorChannel := make(chan error, 1)
	go startHTTPServer(httpServer, serverErrorChannel)

	logger.Printf("openwrt-controller listening on %s", serverConfiguration.ListenAddress)

	select {
	case receivedSignal := <-shutdownSignalChannel:
		logger.Printf("received signal %s; starting graceful shutdown", receivedSignal)
	case receivedServerError := <-serverErrorChannel:
		logger.Printf("http server stopped with error: %v", receivedServerError)
	}

	shutdownContext, cancelShutdownContext := context.WithTimeout(context.Background(), httpServerShutdownTimeout)
	shutdownError := httpServer.Shutdown(shutdownContext)
	cancelShutdownContext()

	if shutdownError != nil {
		logger.Printf("graceful shutdown failed: %v", shutdownError)
	}
}

func startHTTPServer(httpServer *http.Server, serverErrorChannel chan error) {
	listenError := httpServer.ListenAndServe()
	serverErrorChannel <- listenError
}
