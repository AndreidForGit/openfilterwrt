// Configuration for the whole service.
//
// Everything is configured with environment variables, which is the same idea as
// os.environ in Python. Keeping the settings outside the source code means the
// same program can run in different places (a container, your laptop, ...) with
// different values, and secrets never end up in the repository.
//
// The entry point is LoadServerConfigurationFromEnvironment, which returns a
// ServerConfiguration value that holds every setting the rest of the program
// needs.
//
// Python equivalent: ServerConfiguration is like a dataclass; every field is one
// setting.

package main

import (
	"errors"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	defaultListenAddress       = ":8080"
	defaultStaticDirectory     = "/app/static"
	defaultRouterSSHPort       = "22"
	defaultDNSMasqServiceName  = "dnsmasq"
	defaultSSHTimeoutSeconds   = 10
	defaultEnvironmentFilePath = ".env"
)

// ServerConfiguration holds every value required to run the controller service.
type ServerConfiguration struct {
	EnvironmentFilePath        string
	ListenAddress              string
	StaticDirectory            string
	APIBearerToken             string
	RouterAddress              string
	RouterUsername             string
	RouterPassword             string
	RouterPrivateKey           []byte
	RouterPrivateKeyPath       string
	RouterPrivateKeyPassphrase string
	RouterHostKeyFingerprint   string
	RouterConnectionTimeout    time.Duration
	DNSMasqServiceName         string
	RouterWirelessSections     []string
}

// LoadServerConfigurationFromEnvironment builds a fully populated
// ServerConfiguration value.
//
// Before reading the settings it loads an optional ".env" file (see dotenv.go).
// The path comes from the ENV_FILE_PATH variable and defaults to ".env". Values
// that already exist in the environment always win over the file.
func LoadServerConfigurationFromEnvironment() (ServerConfiguration, error) {
	serverConfiguration := ServerConfiguration{}

	serverConfiguration.EnvironmentFilePath = readEnvironmentVariableOrDefault("ENV_FILE_PATH", defaultEnvironmentFilePath)

	environmentValues, environmentFileError := LoadEnvironmentFile(serverConfiguration.EnvironmentFilePath)
	if environmentFileError != nil {
		return ServerConfiguration{}, environmentFileError
	}
	ApplyEnvironmentValues(environmentValues)

	serverConfiguration.ListenAddress = readEnvironmentVariableOrDefault("LISTEN_ADDRESS", defaultListenAddress)
	serverConfiguration.StaticDirectory = readEnvironmentVariableOrDefault("STATIC_DIRECTORY", defaultStaticDirectory)
	serverConfiguration.APIBearerToken = os.Getenv("API_BEARER_TOKEN")

	routerAddress := os.Getenv("ROUTER_ADDRESS")
	if routerAddress == "" {
		missingAddressError := errors.New("environment variable ROUTER_ADDRESS is required")
		return ServerConfiguration{}, missingAddressError
	}
	serverConfiguration.RouterAddress = ensurePortPresent(routerAddress, defaultRouterSSHPort)

	routerUsername := os.Getenv("ROUTER_USERNAME")
	if routerUsername == "" {
		missingUsernameError := errors.New("environment variable ROUTER_USERNAME is required")
		return ServerConfiguration{}, missingUsernameError
	}
	serverConfiguration.RouterUsername = routerUsername

	serverConfiguration.RouterPassword = os.Getenv("ROUTER_PASSWORD")
	serverConfiguration.RouterPrivateKeyPath = os.Getenv("ROUTER_PRIVATE_KEY_PATH")
	serverConfiguration.RouterPrivateKeyPassphrase = os.Getenv("ROUTER_PRIVATE_KEY_PASSPHRASE")
	serverConfiguration.RouterHostKeyFingerprint = os.Getenv("ROUTER_HOST_KEY_FINGERPRINT")

	inlinePrivateKey := os.Getenv("ROUTER_PRIVATE_KEY")
	if inlinePrivateKey != "" {
		serverConfiguration.RouterPrivateKey = []byte(inlinePrivateKey)
	}

	serverConfiguration.DNSMasqServiceName = readEnvironmentVariableOrDefault("DNSMASQ_SERVICE_NAME", defaultDNSMasqServiceName)

	sshTimeoutSeconds := readIntegerEnvironmentVariableOrDefault("ROUTER_SSH_TIMEOUT_SECONDS", defaultSSHTimeoutSeconds)
	serverConfiguration.RouterConnectionTimeout = time.Duration(sshTimeoutSeconds) * time.Second

	wirelessSectionsValue := os.Getenv("ROUTER_WIRELESS_SECTIONS")
	serverConfiguration.RouterWirelessSections = splitCommaSeparatedValues(wirelessSectionsValue)

	return serverConfiguration, nil
}

// readEnvironmentVariableOrDefault returns the value of an environment variable,
// or the fallback value when the variable is missing or empty.
func readEnvironmentVariableOrDefault(environmentVariableName string, fallbackValue string) string {
	environmentVariableValue := os.Getenv(environmentVariableName)
	if environmentVariableValue == "" {
		return fallbackValue
	}
	return environmentVariableValue
}

// readIntegerEnvironmentVariableOrDefault is the same idea as
// readEnvironmentVariableOrDefault, but it converts the text to a number. When
// the text is not a valid number it silently falls back to the default.
func readIntegerEnvironmentVariableOrDefault(environmentVariableName string, fallbackValue int) int {
	environmentVariableValue := os.Getenv(environmentVariableName)
	if environmentVariableValue == "" {
		return fallbackValue
	}

	parsedValue, parseError := strconv.Atoi(environmentVariableValue)
	if parseError != nil {
		return fallbackValue
	}

	return parsedValue
}

// ensurePortPresent makes sure the router address contains a port. The SSH
// default port is 22, so "192.168.1.1" becomes "192.168.1.1:22", while an address
// that already has a port is returned unchanged.
func ensurePortPresent(routerAddress string, fallbackPort string) string {
	_, _, splitError := net.SplitHostPort(routerAddress)
	if splitError == nil {
		return routerAddress
	}

	addressWithPort := net.JoinHostPort(routerAddress, fallbackPort)
	return addressWithPort
}

// splitCommaSeparatedValues turns "a, b , c" into ["a", "b", "c"] and returns nil
// for an empty string. Python equivalent: [part.strip() for part in value.split(",")].
func splitCommaSeparatedValues(rawValue string) []string {
	trimmedValue := strings.TrimSpace(rawValue)
	if trimmedValue == "" {
		return nil
	}

	rawParts := strings.Split(trimmedValue, ",")
	cleanedValues := make([]string, 0)
	for partIndex := 0; partIndex < len(rawParts); partIndex++ {
		cleanedPart := strings.TrimSpace(rawParts[partIndex])
		if cleanedPart == "" {
			continue
		}
		cleanedValues = append(cleanedValues, cleanedPart)
	}

	return cleanedValues
}
