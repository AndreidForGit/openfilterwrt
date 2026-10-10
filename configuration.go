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
	defaultListenAddress      = ":8080"
	defaultStaticDirectory    = "/app/static"
	defaultRouterSSHPort      = "22"
	defaultDNSMasqServiceName = "dnsmasq"
	defaultSSHTimeoutSeconds  = 10
)

// ServerConfiguration holds every value required to run the controller service.
type ServerConfiguration struct {
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

// LoadServerConfigurationFromEnvironment reads the process environment and
// builds a fully populated ServerConfiguration value.
func LoadServerConfigurationFromEnvironment() (ServerConfiguration, error) {
	serverConfiguration := ServerConfiguration{}

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

func readEnvironmentVariableOrDefault(environmentVariableName string, fallbackValue string) string {
	environmentVariableValue := os.Getenv(environmentVariableName)
	if environmentVariableValue == "" {
		return fallbackValue
	}
	return environmentVariableValue
}

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

func ensurePortPresent(routerAddress string, fallbackPort string) string {
	_, _, splitError := net.SplitHostPort(routerAddress)
	if splitError == nil {
		return routerAddress
	}

	addressWithPort := net.JoinHostPort(routerAddress, fallbackPort)
	return addressWithPort
}

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
