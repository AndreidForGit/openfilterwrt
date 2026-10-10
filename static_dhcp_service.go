package main

import (
	"errors"
	"fmt"
	"sort"
	"sync"
)

const (
	dhcpConfigurationName  = "dhcp"
	dhcpHostSectionType    = "host"
	dhcpHostNameOptionName = "name"
	dhcpHostMACOptionName  = "mac"
	dhcpHostIPOptionName   = "ip"
)

// StaticDHCPHostNotFoundError is returned when a requested reservation is absent.
var StaticDHCPHostNotFoundError = errors.New("static DHCP host was not found")

// StaticDHCPHost represents a static DHCP reservation.
type StaticDHCPHost struct {
	Name       string `json:"name"`
	MACAddress string `json:"macAddress"`
	IPAddress  string `json:"ipAddress"`
}

// StaticDHCPManager defines the operations exposed over the HTTP API.
type StaticDHCPManager interface {
	ListStaticDHCPHosts() ([]StaticDHCPHost, error)
	CreateStaticDHCPHost(newHost StaticDHCPHost) error
	DeleteStaticDHCPHost(macAddress string) error
}

type staticDHCPHostEntry struct {
	SectionIdentifier string
	Host              StaticDHCPHost
}

// StaticDHCPService manages static DHCP reservations on the router.
type StaticDHCPService struct {
	uciClient          *UCIClient
	dnsMasqServiceName string
	mutationLock       sync.Mutex
}

// NewStaticDHCPService creates a StaticDHCPService for the provided UCIClient.
func NewStaticDHCPService(uciClient *UCIClient, dnsMasqServiceName string) *StaticDHCPService {
	staticDHCPService := &StaticDHCPService{
		uciClient:          uciClient,
		dnsMasqServiceName: dnsMasqServiceName,
	}
	return staticDHCPService
}

// ListStaticDHCPHosts returns every static DHCP reservation sorted by IP address.
func (service *StaticDHCPService) ListStaticDHCPHosts() ([]StaticDHCPHost, error) {
	hostEntries, listError := service.listStaticDHCPHostEntries()
	if listError != nil {
		return nil, listError
	}

	staticDHCPHosts := make([]StaticDHCPHost, 0, len(hostEntries))
	for entryIndex := 0; entryIndex < len(hostEntries); entryIndex++ {
		staticDHCPHosts = append(staticDHCPHosts, hostEntries[entryIndex].Host)
	}

	sort.Sort(staticDHCPHostCollection(staticDHCPHosts))
	return staticDHCPHosts, nil
}

// CreateStaticDHCPHost creates a new static DHCP reservation.
func (service *StaticDHCPService) CreateStaticDHCPHost(newHost StaticDHCPHost) error {
	validationError := validateStaticDHCPHost(newHost)
	if validationError != nil {
		return validationError
	}

	canonicalMACAddress, normalizeError := NormalizeMACAddress(newHost.MACAddress)
	if normalizeError != nil {
		return normalizeError
	}
	newHost.MACAddress = canonicalMACAddress

	service.mutationLock.Lock()
	defer service.mutationLock.Unlock()

	hostEntries, listError := service.listStaticDHCPHostEntries()
	if listError != nil {
		return listError
	}

	for entryIndex := 0; entryIndex < len(hostEntries); entryIndex++ {
		existingHost := hostEntries[entryIndex].Host
		macAddressMatches := macAddressesEqual(existingHost.MACAddress, newHost.MACAddress)
		if macAddressMatches {
			errorMessage := fmt.Sprintf("a static DHCP host for MAC address %s already exists", newHost.MACAddress)
			duplicateHostError := errors.New(errorMessage)
			return duplicateHostError
		}
	}

	sectionIdentifier, addSectionError := service.uciClient.AddSection(dhcpConfigurationName, dhcpHostSectionType)
	if addSectionError != nil {
		return addSectionError
	}

	setNameError := service.uciClient.SetOption(dhcpConfigurationName, sectionIdentifier, dhcpHostNameOptionName, newHost.Name)
	if setNameError != nil {
		return setNameError
	}

	setMACError := service.uciClient.SetOption(dhcpConfigurationName, sectionIdentifier, dhcpHostMACOptionName, newHost.MACAddress)
	if setMACError != nil {
		return setMACError
	}

	setIPError := service.uciClient.SetOption(dhcpConfigurationName, sectionIdentifier, dhcpHostIPOptionName, newHost.IPAddress)
	if setIPError != nil {
		return setIPError
	}

	commitError := service.uciClient.Commit(dhcpConfigurationName)
	if commitError != nil {
		return commitError
	}

	restartError := service.uciClient.RestartService(service.dnsMasqServiceName)
	if restartError != nil {
		return restartError
	}

	return nil
}

// DeleteStaticDHCPHost removes the static DHCP reservation matching a MAC address.
func (service *StaticDHCPService) DeleteStaticDHCPHost(macAddress string) error {
	validationError := ValidateMACAddress(macAddress)
	if validationError != nil {
		return validationError
	}

	service.mutationLock.Lock()
	defer service.mutationLock.Unlock()

	hostEntries, listError := service.listStaticDHCPHostEntries()
	if listError != nil {
		return listError
	}

	matchedSectionIdentifier := ""
	for entryIndex := 0; entryIndex < len(hostEntries); entryIndex++ {
		currentEntry := hostEntries[entryIndex]
		macAddressMatches := macAddressesEqual(currentEntry.Host.MACAddress, macAddress)
		if macAddressMatches {
			matchedSectionIdentifier = currentEntry.SectionIdentifier
			break
		}
	}

	if matchedSectionIdentifier == "" {
		return StaticDHCPHostNotFoundError
	}

	deleteError := service.uciClient.DeleteSection(dhcpConfigurationName, matchedSectionIdentifier)
	if deleteError != nil {
		return deleteError
	}

	commitError := service.uciClient.Commit(dhcpConfigurationName)
	if commitError != nil {
		return commitError
	}

	restartError := service.uciClient.RestartService(service.dnsMasqServiceName)
	if restartError != nil {
		return restartError
	}

	return nil
}

func (service *StaticDHCPService) listStaticDHCPHostEntries() ([]staticDHCPHostEntry, error) {
	sections, showError := service.uciClient.ShowConfiguration(dhcpConfigurationName)
	if showError != nil {
		return nil, showError
	}

	hostEntries := make([]staticDHCPHostEntry, 0)
	for sectionIdentifier, section := range sections {
		if section.SectionType != dhcpHostSectionType {
			continue
		}

		currentHost := StaticDHCPHost{
			Name:       firstOptionValue(section, dhcpHostNameOptionName),
			MACAddress: firstOptionValue(section, dhcpHostMACOptionName),
			IPAddress:  firstOptionValue(section, dhcpHostIPOptionName),
		}
		currentEntry := staticDHCPHostEntry{
			SectionIdentifier: sectionIdentifier,
			Host:              currentHost,
		}
		hostEntries = append(hostEntries, currentEntry)
	}

	return hostEntries, nil
}

func validateStaticDHCPHost(hostToValidate StaticDHCPHost) error {
	macValidationError := ValidateMACAddress(hostToValidate.MACAddress)
	if macValidationError != nil {
		return macValidationError
	}

	ipValidationError := ValidateIPv4Address(hostToValidate.IPAddress)
	if ipValidationError != nil {
		return ipValidationError
	}

	if hostToValidate.Name == "" {
		nameValidationError := &ValidationError{Message: "static DHCP host name is required"}
		return nameValidationError
	}

	return nil
}

type staticDHCPHostCollection []StaticDHCPHost

func (collection staticDHCPHostCollection) Len() int {
	return len(collection)
}

func (collection staticDHCPHostCollection) Swap(leftIndex int, rightIndex int) {
	leftValue := collection[leftIndex]
	rightValue := collection[rightIndex]
	collection[leftIndex] = rightValue
	collection[rightIndex] = leftValue
}

func (collection staticDHCPHostCollection) Less(leftIndex int, rightIndex int) bool {
	leftHost := collection[leftIndex]
	rightHost := collection[rightIndex]
	if leftHost.IPAddress != rightHost.IPAddress {
		return leftHost.IPAddress < rightHost.IPAddress
	}
	return leftHost.MACAddress < rightHost.MACAddress
}
