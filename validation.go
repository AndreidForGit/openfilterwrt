package main

import (
	"bytes"
	"fmt"
	"net"
	"strings"
)

// ValidationError represents an input value that was rejected before any
// router command was executed.
type ValidationError struct {
	Message string
}

func (validationError *ValidationError) Error() string {
	return validationError.Message
}

// ValidateMACAddress confirms that the provided text is a well formed MAC address.
func ValidateMACAddress(macAddress string) error {
	_, parseError := net.ParseMAC(macAddress)
	if parseError != nil {
		errorMessage := fmt.Sprintf("invalid MAC address %q", macAddress)
		validationError := &ValidationError{Message: errorMessage}
		return validationError
	}

	return nil
}

// ValidateIPv4Address confirms that the provided text is a well formed IPv4 address.
func ValidateIPv4Address(ipAddress string) error {
	parsedAddress := net.ParseIP(ipAddress)
	if parsedAddress == nil {
		errorMessage := fmt.Sprintf("invalid IP address %q", ipAddress)
		validationError := &ValidationError{Message: errorMessage}
		return validationError
	}

	parsedIPv4Address := parsedAddress.To4()
	if parsedIPv4Address == nil {
		errorMessage := fmt.Sprintf("only IPv4 addresses are supported, received %q", ipAddress)
		validationError := &ValidationError{Message: errorMessage}
		return validationError
	}

	return nil
}

// macAddressesEqual compares two MAC addresses regardless of their textual
// formatting (case and separator style).
func macAddressesEqual(firstMACAddress string, secondMACAddress string) bool {
	firstHardwareAddress, firstParseError := net.ParseMAC(firstMACAddress)
	secondHardwareAddress, secondParseError := net.ParseMAC(secondMACAddress)

	if firstParseError != nil {
		return strings.EqualFold(firstMACAddress, secondMACAddress)
	}

	if secondParseError != nil {
		return strings.EqualFold(firstMACAddress, secondMACAddress)
	}

	addressesMatch := bytes.Equal(firstHardwareAddress, secondHardwareAddress)
	return addressesMatch
}
