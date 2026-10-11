// Tests for the UCI client and for the parser of "uci show" output.
//
// The sample outputs below are the real text that "uci show" prints on a router.
// Notice that a list option such as "maclist" is printed as several quoted values
// on one single line, which is exactly what parseUCIValueTokens has to split.

package main

import (
	"strings"
	"testing"
)

const sampleUCIHostOutput = `dhcp.@host[0]=host
dhcp.@host[0].name='laptop'
dhcp.@host[0].mac='AA:BB:CC:DD:EE:FF'
dhcp.@host[0].ip='192.168.1.100'
dhcp.@host[1]=host
dhcp.@host[1].name='phone'
dhcp.@host[1].mac='11:22:33:44:55:66'
dhcp.@host[1].ip='192.168.1.101'
`

const sampleUCIWirelessOutput = `wireless.radio0=wifi-device
wireless.radio0.type='mac80211'
wireless.default_radio0=wifi-iface
wireless.default_radio0.device='radio0'
wireless.default_radio0.mode='ap'
wireless.default_radio0.macfilter='allow'
wireless.default_radio0.maclist='AA:BB:CC:DD:EE:FF' '11:22:33:44:55:66'
`

const sampleUCIMultipleMACListOutput = `wireless.default_radio0=wifi-iface
wireless.default_radio0.mode='ap'
wireless.default_radio0.macfilter='allow'
wireless.default_radio0.maclist='AA:BB:CC:DD:EE:01' 'AA:BB:CC:DD:EE:02' 'AA:BB:CC:DD:EE:03' 'AA:BB:CC:DD:EE:04' 'AA:BB:CC:DD:EE:05' 'AA:BB:CC:DD:EE:06'
`

func TestParseUCIShowOutputSections(testingInstance *testing.T) {
	sectionMap := ParseUCIShowOutput(sampleUCIHostOutput)

	if len(sectionMap) != 2 {
		testingInstance.Fatalf("expected 2 sections, received %d", len(sectionMap))
	}

	firstSection, firstSectionExists := sectionMap["@host[0]"]
	if firstSectionExists == false {
		testingInstance.Fatalf("expected section @host[0] to exist")
		return
	}

	if firstSection.SectionType != "host" {
		testingInstance.Errorf("expected section type host, received %q", firstSection.SectionType)
	}

	firstHostName := firstOptionValue(firstSection, "name")
	if firstHostName != "laptop" {
		testingInstance.Errorf("expected name laptop, received %q", firstHostName)
	}
}

func TestParseUCIShowOutputListOption(testingInstance *testing.T) {
	sectionMap := ParseUCIShowOutput(sampleUCIWirelessOutput)

	wirelessSection, wirelessSectionExists := sectionMap["default_radio0"]
	if wirelessSectionExists == false {
		testingInstance.Fatalf("expected section default_radio0 to exist")
		return
	}

	listedMACAddresses, listExists := wirelessSection.Options["maclist"]
	if listExists == false {
		testingInstance.Fatalf("expected maclist option to exist")
		return
	}

	if len(listedMACAddresses) != 2 {
		testingInstance.Errorf("expected 2 maclist values, received %d", len(listedMACAddresses))
	}
}

func TestBuildShellQuotedValue(testingInstance *testing.T) {
	quotedValue := BuildShellQuotedValue("plain value")
	if quotedValue != "'plain value'" {
		testingInstance.Errorf("unexpected quoting: %s", quotedValue)
	}

	escapedValue := BuildShellQuotedValue("it's fine")
	expectedEscapedValue := "'it'\\''s fine'"
	if escapedValue != expectedEscapedValue {
		testingInstance.Errorf("expected %s, received %s", expectedEscapedValue, escapedValue)
	}
}

func TestUCIClientReportsCommandFailure(testingInstance *testing.T) {
	fakeExecutor := newFakeCommandExecutor()
	fakeExecutor.commandResults["uci commit dhcp"] = CommandResult{
		StandardError: "uci: invalid command",
		ExitStatus:    1,
	}

	uciClient := NewUCIClient(fakeExecutor)
	commitError := uciClient.Commit("dhcp")
	if commitError == nil {
		testingInstance.Fatalf("expected commit to fail")
	}

	if strings.Contains(commitError.Error(), "exit status 1") == false {
		testingInstance.Errorf("expected error to mention exit status, received %q", commitError.Error())
	}
}

func TestParseUCIValueTokens(testingInstance *testing.T) {
	multipleTokens := parseUCIValueTokens("'AA:BB:CC:DD:EE:FF' '11:22:33:44:55:66'")
	if len(multipleTokens) != 2 {
		testingInstance.Fatalf("expected 2 tokens, received %d: %v", len(multipleTokens), multipleTokens)
	}
	if multipleTokens[0] != "AA:BB:CC:DD:EE:FF" {
		testingInstance.Errorf("unexpected first token %q", multipleTokens[0])
	}
	if multipleTokens[1] != "11:22:33:44:55:66" {
		testingInstance.Errorf("unexpected second token %q", multipleTokens[1])
	}

	singleToken := parseUCIValueTokens("'ap'")
	if len(singleToken) != 1 {
		testingInstance.Fatalf("expected 1 token, received %d: %v", len(singleToken), singleToken)
	}
	if singleToken[0] != "ap" {
		testingInstance.Errorf("unexpected single token %q", singleToken[0])
	}

	unquotedToken := parseUCIValueTokens("wifi-iface")
	if len(unquotedToken) != 1 {
		testingInstance.Fatalf("expected 1 unquoted token, received %d: %v", len(unquotedToken), unquotedToken)
	}
	if unquotedToken[0] != "wifi-iface" {
		testingInstance.Errorf("unexpected unquoted token %q", unquotedToken[0])
	}

	emptyToken := parseUCIValueTokens("''")
	if len(emptyToken) != 1 {
		testingInstance.Fatalf("expected 1 empty token, received %d: %v", len(emptyToken), emptyToken)
	}
	if emptyToken[0] != "" {
		testingInstance.Errorf("expected an empty token, received %q", emptyToken[0])
	}
}

func TestParseUCIShowOutputMultipleListValues(testingInstance *testing.T) {
	sectionMap := ParseUCIShowOutput(sampleUCIMultipleMACListOutput)

	wirelessSection, wirelessSectionExists := sectionMap["default_radio0"]
	if wirelessSectionExists == false {
		testingInstance.Fatalf("expected section default_radio0 to exist")
		return
	}

	listedMACAddresses, listExists := wirelessSection.Options["maclist"]
	if listExists == false {
		testingInstance.Fatalf("expected the maclist option to exist")
		return
	}

	if len(listedMACAddresses) != 6 {
		testingInstance.Fatalf("expected 6 maclist values, received %d: %v", len(listedMACAddresses), listedMACAddresses)
	}

	if listedMACAddresses[0] != "AA:BB:CC:DD:EE:01" {
		testingInstance.Errorf("expected first value AA:BB:CC:DD:EE:01, received %q", listedMACAddresses[0])
	}

	if listedMACAddresses[5] != "AA:BB:CC:DD:EE:06" {
		testingInstance.Errorf("expected last value AA:BB:CC:DD:EE:06, received %q", listedMACAddresses[5])
	}
}
