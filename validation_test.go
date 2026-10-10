package main

import "testing"

func TestValidateMACAddressAcceptsValidAddresses(testingInstance *testing.T) {
	validMACAddresses := []string{
		"AA:BB:CC:DD:EE:FF",
		"aa:bb:cc:dd:ee:ff",
		"AA-BB-CC-DD-EE-FF",
		"AABBCCDDEEFF",
		"aabbccddeeff",
	}

	for addressIndex := 0; addressIndex < len(validMACAddresses); addressIndex++ {
		currentMACAddress := validMACAddresses[addressIndex]
		validationError := ValidateMACAddress(currentMACAddress)
		if validationError != nil {
			testingInstance.Errorf("expected MAC address %q to be valid, received error: %v", currentMACAddress, validationError)
		}
	}
}

func TestValidateMACAddressRejectsInvalidAddresses(testingInstance *testing.T) {
	invalidMACAddresses := []string{
		"",
		"not-a-mac-address",
		"AA:BB:CC:DD:EE",
		"GG:BB:CC:DD:EE:FF",
	}

	for addressIndex := 0; addressIndex < len(invalidMACAddresses); addressIndex++ {
		currentMACAddress := invalidMACAddresses[addressIndex]
		validationError := ValidateMACAddress(currentMACAddress)
		if validationError == nil {
			testingInstance.Errorf("expected MAC address %q to be invalid", currentMACAddress)
		}
	}
}

func TestValidateIPv4Address(testingInstance *testing.T) {
	validError := ValidateIPv4Address("192.168.1.100")
	if validError != nil {
		testingInstance.Errorf("expected IPv4 address to be valid, received error: %v", validError)
	}

	invalidError := ValidateIPv4Address("not-an-ip-address")
	if invalidError == nil {
		testingInstance.Errorf("expected an invalid IPv4 address to be rejected")
	}

	ipv6Error := ValidateIPv4Address("fd00::1")
	if ipv6Error == nil {
		testingInstance.Errorf("expected an IPv6 address to be rejected")
	}
}

func TestMacAddressesEqual(testingInstance *testing.T) {
	matchingResult := macAddressesEqual("AA:BB:CC:DD:EE:FF", "aa:bb:cc:dd:ee:ff")
	if matchingResult == false {
		testingInstance.Errorf("expected MAC addresses with different casing to be equal")
	}

	differingResult := macAddressesEqual("AA:BB:CC:DD:EE:FF", "AA:BB:CC:DD:EE:00")
	if differingResult {
		testingInstance.Errorf("expected different MAC addresses to be unequal")
	}
}

func TestNormalizeMACAddress(testingInstance *testing.T) {
	compactValue, compactError := NormalizeMACAddress("aabbccddeeff")
	if compactError != nil {
		testingInstance.Fatalf("unexpected normalize error: %v", compactError)
	}
	if compactValue != "AA:BB:CC:DD:EE:FF" {
		testingInstance.Errorf("expected AA:BB:CC:DD:EE:FF, received %q", compactValue)
	}

	separatedValue, separatedError := NormalizeMACAddress("aa-bb-cc-dd-ee-ff")
	if separatedError != nil {
		testingInstance.Fatalf("unexpected normalize error: %v", separatedError)
	}
	if separatedValue != "AA:BB:CC:DD:EE:FF" {
		testingInstance.Errorf("expected AA:BB:CC:DD:EE:FF, received %q", separatedValue)
	}

	_, invalidError := NormalizeMACAddress("not-a-mac-address")
	if invalidError == nil {
		testingInstance.Errorf("expected an invalid MAC address to fail normalization")
	}
}
