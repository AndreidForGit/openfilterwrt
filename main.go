// OpenWrt Mobile Controller
//
// This program is a small web service written in Go. It does three things:
//
//  1. It serves a Progressive Web App (PWA) from the "static" directory.
//  2. It exposes a small JSON HTTP API under the "/api/" path.
//  3. When the API is used, it connects to an OpenWrt router over SSH and runs
//     "uci" commands to change the router configuration.
//
// Two router features are supported: static DHCP reservations and the wireless
// MAC address allow list.
//
// What happens when the program starts (see the main function below):
//
//  1. Read the configuration from environment variables.
//  2. Build the objects that talk to the router over SSH.
//  3. Create the HTTP handlers and start the HTTP server.
//  4. Wait until the operating system asks the program to stop (Ctrl+C or
//     SIGTERM) and then shut down cleanly.
//
// Python equivalent: this file is like the "if __name__ == '__main__':" block of
// a Python program. It only wires the pieces together; the real logic lives in
// the other files.
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

// Timeouts used by the HTTP server. ReadHeaderTimeout protects against very slow
// clients, and ShutdownTimeout caps how long we wait for in-flight requests to
// finish when the program is asked to stop.
const (
	httpServerReadHeaderTimeout = 10 * time.Second
	httpServerShutdownTimeout   = 10 * time.Second
)

// main is the entry point: Go calls it automatically. It is the equivalent of the
// "if __name__ == '__main__':" block in a Python program.
func main() {
	// Step 1: read every setting from the environment variables.
	serverConfiguration, configurationError := LoadServerConfigurationFromEnvironment()
	if configurationError != nil {
		log.Fatalf("failed to load server configuration: %v", configurationError)
	}

	logger := log.New(os.Stdout, "", log.LstdFlags)

	// Without a pinned fingerprint the program cannot tell a real router from a
	// fake one, so we warn the operator about it.
	if serverConfiguration.RouterHostKeyFingerprint == "" {
		logger.Printf("warning: ROUTER_HOST_KEY_FINGERPRINT is not set; SSH host key verification is disabled")
	}

	// Step 2: build the objects that talk to the router over SSH.
	routerConnection := NewRouterConnection(serverConfiguration)
	uciClient := NewUCIClient(routerConnection)

	// Step 3: create the two services, one per router feature.
	staticDHCPService := NewStaticDHCPService(uciClient, serverConfiguration.DNSMasqServiceName)
	macWhitelistService := NewMACWhitelistService(uciClient, serverConfiguration.RouterWirelessSections)

	// Step 4: wrap the services in the HTTP API and start serving.
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

	// Channels are the way Go passes values between goroutines, a bit like a
	// queue. shutdownSignalChannel receives Ctrl+C / SIGTERM from the operating
	// system, and serverErrorChannel receives an error if the server dies.
	shutdownSignalChannel := make(chan os.Signal, 1)
	signal.Notify(shutdownSignalChannel, syscall.SIGINT, syscall.SIGTERM)

	serverErrorChannel := make(chan error, 1)

	// "go" starts a new goroutine, which is a very lightweight thread. The server
	// runs there while the main goroutine keeps going.
	go startHTTPServer(httpServer, serverErrorChannel)

	logger.Printf("openwrt-controller listening on %s", serverConfiguration.ListenAddress)

	// "select" waits until one of the channels has a value, like waiting on two
	// events at the same time. Whichever happens first wins.
	select {
	case receivedSignal := <-shutdownSignalChannel:
		logger.Printf("received signal %s; starting graceful shutdown", receivedSignal)
	case receivedServerError := <-serverErrorChannel:
		logger.Printf("http server stopped with error: %v", receivedServerError)
	}

	// Step 5: shut down cleanly, giving open requests a few seconds to finish.
	shutdownContext, cancelShutdownContext := context.WithTimeout(context.Background(), httpServerShutdownTimeout)
	shutdownError := httpServer.Shutdown(shutdownContext)
	cancelShutdownContext()

	if shutdownError != nil {
		logger.Printf("graceful shutdown failed: %v", shutdownError)
	}
}

// startHTTPServer runs the blocking ListenAndServe call. It is meant to be
// started with "go startHTTPServer(...)" so the main goroutine stays free.
func startHTTPServer(httpServer *http.Server, serverErrorChannel chan error) {
	listenError := httpServer.ListenAndServe()
	serverErrorChannel <- listenError
}
