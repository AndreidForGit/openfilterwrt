// Business logic for the wireless MAC address allow list.
//
// OpenWrt can filter Wi-Fi clients by MAC address. When the filter is set to
// "allow", only the MAC addresses listed in "maclist" are allowed to connect.
// That list lives in the "wifi-iface" sections of /etc/config/wireless.
//
// MACWhitelistService adds or removes MAC addresses and reloads the wireless
// configuration so the change takes effect.
//
// Warning: switching to "allow" mode blocks every client that is not on the list,
// so the first address added turns the network into a whitelist-only network.

package main

import (
	"errors"
	"sort"
	"sync"
)

const (
	wirelessConfigurationName    = "wireless"
	wirelessInterfaceSectionType = "wifi-iface"
	wirelessModeOptionName       = "mode"
	wirelessAccessPointModeValue = "ap"
	wirelessMACFilterOptionName  = "macfilter"
	wirelessMACFilterAllowValue  = "allow"
	wirelessMACListOptionName    = "maclist"
)

// MACWhitelistEntryNotFoundError is returned when a MAC address is absent.
var MACWhitelistEntryNotFoundError = errors.New("MAC whitelist entry was not found")

// MACWhitelistEntry represents one allowed wireless client MAC address.
type MACWhitelistEntry struct {
	MACAddress string `json:"macAddress"`
}

// MACWhitelistManager defines the whitelist operations exposed over the HTTP API.
type MACWhitelistManager interface {
	ListMACWhitelistEntries() ([]MACWhitelistEntry, error)
	AddMACWhitelistEntry(macAddress string) error
	RemoveMACWhitelistEntry(macAddress string) error
}

// MACWhitelistService manages the wireless MAC address allow list.
type MACWhitelistService struct {
	uciClient                    *UCIClient
	configuredSectionIdentifiers []string
	mutationLock                 sync.Mutex
}

// NewMACWhitelistService creates a MACWhitelistService for the provided UCIClient.
func NewMACWhitelistService(uciClient *UCIClient, configuredSectionIdentifiers []string) *MACWhitelistService {
	macWhitelistService := &MACWhitelistService{
		uciClient:                    uciClient,
		configuredSectionIdentifiers: configuredSectionIdentifiers,
	}
	return macWhitelistService
}

// ListMACWhitelistEntries returns every allowed MAC address, without duplicates.
func (service *MACWhitelistService) ListMACWhitelistEntries() ([]MACWhitelistEntry, error) {
	sections, showError := service.uciClient.ShowConfiguration(wirelessConfigurationName)
	if showError != nil {
		return nil, showError
	}

	targetSectionIdentifiers := service.targetSectionIdentifiers(sections)

	whitelistEntries := make([]MACWhitelistEntry, 0)
	seenMACAddresses := make(map[string]bool)

	for sectionIndex := 0; sectionIndex < len(targetSectionIdentifiers); sectionIndex++ {
		sectionIdentifier := targetSectionIdentifiers[sectionIndex]
		section, sectionExists := sections[sectionIdentifier]
		if sectionExists == false {
			continue
		}

		storedMACAddresses, optionExists := section.Options[wirelessMACListOptionName]
		if optionExists == false {
			continue
		}

		for valueIndex := 0; valueIndex < len(storedMACAddresses); valueIndex++ {
			storedMACAddress := storedMACAddresses[valueIndex]
			alreadySeen := seenMACAddresses[storedMACAddress]
			if alreadySeen {
				continue
			}
			seenMACAddresses[storedMACAddress] = true
			newEntry := MACWhitelistEntry{MACAddress: storedMACAddress}
			whitelistEntries = append(whitelistEntries, newEntry)
		}
	}

	sort.Sort(macWhitelistEntryCollection(whitelistEntries))
	return whitelistEntries, nil
}

// AddMACWhitelistEntry allows a MAC address on every target access point.
// This also switches the wireless filter into allow (whitelist) mode.
func (service *MACWhitelistService) AddMACWhitelistEntry(macAddress string) error {
	canonicalMACAddress, normalizeError := NormalizeMACAddress(macAddress)
	if normalizeError != nil {
		return normalizeError
	}

	service.mutationLock.Lock()
	defer service.mutationLock.Unlock()

	sections, showError := service.uciClient.ShowConfiguration(wirelessConfigurationName)
	if showError != nil {
		return showError
	}

	targetSectionIdentifiers := service.targetSectionIdentifiers(sections)
	if len(targetSectionIdentifiers) == 0 {
		noSectionsError := errors.New("no target wireless access point sections were found")
		return noSectionsError
	}

	for sectionIndex := 0; sectionIndex < len(targetSectionIdentifiers); sectionIndex++ {
		sectionIdentifier := targetSectionIdentifiers[sectionIndex]

		setFilterModeError := service.uciClient.SetOption(wirelessConfigurationName, sectionIdentifier, wirelessMACFilterOptionName, wirelessMACFilterAllowValue)
		if setFilterModeError != nil {
			return setFilterModeError
		}

		section, sectionExists := sections[sectionIdentifier]
		alreadyAllowed := false
		if sectionExists {
			alreadyAllowed = sectionContainsMACAddress(section, wirelessMACListOptionName, canonicalMACAddress)
		}
		if alreadyAllowed {
			continue
		}

		addListValueError := service.uciClient.AddListValue(wirelessConfigurationName, sectionIdentifier, wirelessMACListOptionName, canonicalMACAddress)
		if addListValueError != nil {
			return addListValueError
		}
	}

	commitError := service.uciClient.Commit(wirelessConfigurationName)
	if commitError != nil {
		return commitError
	}

	reloadError := service.uciClient.ReloadWireless()
	if reloadError != nil {
		return reloadError
	}

	return nil
}

// RemoveMACWhitelistEntry revokes a MAC address from every target access point.
func (service *MACWhitelistService) RemoveMACWhitelistEntry(macAddress string) error {
	validationError := ValidateMACAddress(macAddress)
	if validationError != nil {
		return validationError
	}

	service.mutationLock.Lock()
	defer service.mutationLock.Unlock()

	sections, showError := service.uciClient.ShowConfiguration(wirelessConfigurationName)
	if showError != nil {
		return showError
	}

	targetSectionIdentifiers := service.targetSectionIdentifiers(sections)

	removedFromAnySection := false
	for sectionIndex := 0; sectionIndex < len(targetSectionIdentifiers); sectionIndex++ {
		sectionIdentifier := targetSectionIdentifiers[sectionIndex]
		section, sectionExists := sections[sectionIdentifier]
		if sectionExists == false {
			continue
		}

		storedMACAddresses, optionExists := section.Options[wirelessMACListOptionName]
		if optionExists == false {
			continue
		}

		for valueIndex := 0; valueIndex < len(storedMACAddresses); valueIndex++ {
			storedMACAddress := storedMACAddresses[valueIndex]
			macAddressMatches := macAddressesEqual(storedMACAddress, macAddress)
			if macAddressMatches == false {
				continue
			}

			removeListValueError := service.uciClient.DeleteListValue(wirelessConfigurationName, sectionIdentifier, wirelessMACListOptionName, storedMACAddress)
			if removeListValueError != nil {
				return removeListValueError
			}
			removedFromAnySection = true
		}
	}

	if removedFromAnySection == false {
		return MACWhitelistEntryNotFoundError
	}

	commitError := service.uciClient.Commit(wirelessConfigurationName)
	if commitError != nil {
		return commitError
	}

	reloadError := service.uciClient.ReloadWireless()
	if reloadError != nil {
		return reloadError
	}

	return nil
}

// targetSectionIdentifiers decides which wifi-iface sections will be modified: the
// ones listed in ROUTER_WIRELESS_SECTIONS, or every section of type "wifi-iface"
// whose "mode" option is "ap" when that setting is empty.
func (service *MACWhitelistService) targetSectionIdentifiers(sections map[string]*UCISection) []string {
	configuredSectionIdentifiers := service.configuredSectionIdentifiers
	if len(configuredSectionIdentifiers) > 0 {
		targetSectionIdentifiers := make([]string, len(configuredSectionIdentifiers))
		copy(targetSectionIdentifiers, configuredSectionIdentifiers)
		return targetSectionIdentifiers
	}

	targetSectionIdentifiers := make([]string, 0)
	for sectionIdentifier, section := range sections {
		sectionTypeMatches := section.SectionType == wirelessInterfaceSectionType
		if sectionTypeMatches == false {
			continue
		}

		sectionMode := firstOptionValue(section, wirelessModeOptionName)
		accessPointModeMatches := sectionMode == wirelessAccessPointModeValue
		if accessPointModeMatches == false {
			continue
		}

		targetSectionIdentifiers = append(targetSectionIdentifiers, sectionIdentifier)
	}

	sort.Strings(targetSectionIdentifiers)
	return targetSectionIdentifiers
}

// sectionContainsMACAddress reports whether an option already contains a given MAC
// address. It is used to avoid adding the same address twice, and it compares the
// addresses in a tolerant way (upper/lower case and separators do not matter).
func sectionContainsMACAddress(section *UCISection, optionName string, macAddress string) bool {
	optionValues, optionExists := section.Options[optionName]
	if optionExists == false {
		return false
	}

	for valueIndex := 0; valueIndex < len(optionValues); valueIndex++ {
		if macAddressesEqual(optionValues[valueIndex], macAddress) {
			return true
		}
	}

	return false
}

// macWhitelistEntryCollection teaches Go how to sort the allow list by MAC address.
type macWhitelistEntryCollection []MACWhitelistEntry

func (collection macWhitelistEntryCollection) Len() int {
	return len(collection)
}

func (collection macWhitelistEntryCollection) Swap(leftIndex int, rightIndex int) {
	leftValue := collection[leftIndex]
	rightValue := collection[rightIndex]
	collection[leftIndex] = rightValue
	collection[rightIndex] = leftValue
}

func (collection macWhitelistEntryCollection) Less(leftIndex int, rightIndex int) bool {
	return collection[leftIndex].MACAddress < collection[rightIndex].MACAddress
}
