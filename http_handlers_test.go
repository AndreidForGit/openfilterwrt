// Tests for the HTTP handlers.
//
// Fake managers replace the real services, so these tests focus on routing, HTTP
// status codes and JSON bodies without touching a router.

package main

import (
	"errors"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type fakeStaticDHCPManager struct {
	hosts             []StaticDHCPHost
	listError         error
	createError       error
	deleteError       error
	deletedMACAddress string
}

func (manager *fakeStaticDHCPManager) ListStaticDHCPHosts() ([]StaticDHCPHost, error) {
	if manager.listError != nil {
		return nil, manager.listError
	}
	return manager.hosts, nil
}

func (manager *fakeStaticDHCPManager) CreateStaticDHCPHost(newHost StaticDHCPHost) error {
	return manager.createError
}

func (manager *fakeStaticDHCPManager) DeleteStaticDHCPHost(macAddress string) error {
	manager.deletedMACAddress = macAddress
	return manager.deleteError
}

type fakeMACWhitelistManager struct {
	entries           []MACWhitelistEntry
	listError         error
	addError          error
	removeError       error
	addedMACAddress   string
	removedMACAddress string
}

func (manager *fakeMACWhitelistManager) ListMACWhitelistEntries() ([]MACWhitelistEntry, error) {
	if manager.listError != nil {
		return nil, manager.listError
	}
	return manager.entries, nil
}

func (manager *fakeMACWhitelistManager) AddMACWhitelistEntry(macAddress string) error {
	manager.addedMACAddress = macAddress
	return manager.addError
}

func (manager *fakeMACWhitelistManager) RemoveMACWhitelistEntry(macAddress string) error {
	manager.removedMACAddress = macAddress
	return manager.removeError
}

func newTestHTTPHandler(application *Application, bearerToken string) http.Handler {
	testLogger := log.New(io.Discard, "", 0)
	httpHandler := NewHTTPHandler(application, "static", bearerToken, testLogger)
	return httpHandler
}

func TestHealthEndpoint(testingInstance *testing.T) {
	application := &Application{
		StaticDHCPManager:   &fakeStaticDHCPManager{},
		MACWhitelistManager: &fakeMACWhitelistManager{},
	}
	httpHandler := newTestHTTPHandler(application, "")

	request := httptest.NewRequest("GET", "/api/health", nil)
	responseRecorder := httptest.NewRecorder()
	httpHandler.ServeHTTP(responseRecorder, request)

	if responseRecorder.Code != http.StatusOK {
		testingInstance.Errorf("expected status 200, received %d", responseRecorder.Code)
	}

	responseBody := responseRecorder.Body.String()
	if strings.Contains(responseBody, "\"ok\"") == false {
		testingInstance.Errorf("expected ok status in body, received %q", responseBody)
	}
}

func TestListStaticDHCPHostsEndpoint(testingInstance *testing.T) {
	fakeManager := &fakeStaticDHCPManager{
		hosts: []StaticDHCPHost{
			{
				Name:       "laptop",
				MACAddress: "AA:BB:CC:DD:EE:FF",
				IPAddress:  "192.168.1.100",
			},
		},
	}
	application := &Application{
		StaticDHCPManager:   fakeManager,
		MACWhitelistManager: &fakeMACWhitelistManager{},
	}
	httpHandler := newTestHTTPHandler(application, "")

	request := httptest.NewRequest("GET", "/api/static-dhcp-hosts", nil)
	responseRecorder := httptest.NewRecorder()
	httpHandler.ServeHTTP(responseRecorder, request)

	if responseRecorder.Code != http.StatusOK {
		testingInstance.Errorf("expected status 200, received %d", responseRecorder.Code)
	}

	responseBody := responseRecorder.Body.String()
	if strings.Contains(responseBody, "192.168.1.100") == false {
		testingInstance.Errorf("expected host IP in body, received %q", responseBody)
	}
}

func TestCreateStaticDHCPHostValidationFailure(testingInstance *testing.T) {
	fakeManager := &fakeStaticDHCPManager{
		createError: &ValidationError{Message: "invalid MAC address"},
	}
	application := &Application{
		StaticDHCPManager:   fakeManager,
		MACWhitelistManager: &fakeMACWhitelistManager{},
	}
	httpHandler := newTestHTTPHandler(application, "")

	requestBody := strings.NewReader(`{"name":"laptop","macAddress":"bad","ipAddress":"192.168.1.100"}`)
	request := httptest.NewRequest("POST", "/api/static-dhcp-hosts", requestBody)
	responseRecorder := httptest.NewRecorder()
	httpHandler.ServeHTTP(responseRecorder, request)

	if responseRecorder.Code != http.StatusBadRequest {
		testingInstance.Errorf("expected status 400, received %d", responseRecorder.Code)
	}
}

func TestDeleteStaticDHCPHostNotFound(testingInstance *testing.T) {
	fakeManager := &fakeStaticDHCPManager{
		deleteError: StaticDHCPHostNotFoundError,
	}
	application := &Application{
		StaticDHCPManager:   fakeManager,
		MACWhitelistManager: &fakeMACWhitelistManager{},
	}
	httpHandler := newTestHTTPHandler(application, "")

	request := httptest.NewRequest("DELETE", "/api/static-dhcp-hosts/AA:BB:CC:DD:EE:FF", nil)
	responseRecorder := httptest.NewRecorder()
	httpHandler.ServeHTTP(responseRecorder, request)

	if responseRecorder.Code != http.StatusNotFound {
		testingInstance.Errorf("expected status 404, received %d", responseRecorder.Code)
	}

	if fakeManager.deletedMACAddress != "AA:BB:CC:DD:EE:FF" {
		testingInstance.Errorf("expected path MAC address to be forwarded, received %q", fakeManager.deletedMACAddress)
	}
}

func TestListStaticDHCPHostsUpstreamFailure(testingInstance *testing.T) {
	fakeManager := &fakeStaticDHCPManager{
		listError: errors.New("ssh connection failed"),
	}
	application := &Application{
		StaticDHCPManager:   fakeManager,
		MACWhitelistManager: &fakeMACWhitelistManager{},
	}
	httpHandler := newTestHTTPHandler(application, "")

	request := httptest.NewRequest("GET", "/api/static-dhcp-hosts", nil)
	responseRecorder := httptest.NewRecorder()
	httpHandler.ServeHTTP(responseRecorder, request)

	if responseRecorder.Code != http.StatusBadGateway {
		testingInstance.Errorf("expected status 502, received %d", responseRecorder.Code)
	}
}

func TestBearerTokenProtectsAPI(testingInstance *testing.T) {
	application := &Application{
		StaticDHCPManager:   &fakeStaticDHCPManager{},
		MACWhitelistManager: &fakeMACWhitelistManager{},
	}
	httpHandler := newTestHTTPHandler(application, "secret-token")

	unauthorizedRequest := httptest.NewRequest("GET", "/api/health", nil)
	unauthorizedRecorder := httptest.NewRecorder()
	httpHandler.ServeHTTP(unauthorizedRecorder, unauthorizedRequest)

	if unauthorizedRecorder.Code != http.StatusUnauthorized {
		testingInstance.Errorf("expected status 401 without token, received %d", unauthorizedRecorder.Code)
	}

	authorizedRequest := httptest.NewRequest("GET", "/api/health", nil)
	authorizedRequest.Header.Set("Authorization", "Bearer secret-token")
	authorizedRecorder := httptest.NewRecorder()
	httpHandler.ServeHTTP(authorizedRecorder, authorizedRequest)

	if authorizedRecorder.Code != http.StatusOK {
		testingInstance.Errorf("expected status 200 with token, received %d", authorizedRecorder.Code)
	}
}

func TestListMACWhitelistEndpoint(testingInstance *testing.T) {
	fakeManager := &fakeMACWhitelistManager{
		entries: []MACWhitelistEntry{
			{MACAddress: "AA:BB:CC:DD:EE:FF"},
		},
	}
	application := &Application{
		StaticDHCPManager:   &fakeStaticDHCPManager{},
		MACWhitelistManager: fakeManager,
	}
	httpHandler := newTestHTTPHandler(application, "")

	request := httptest.NewRequest("GET", "/api/mac-whitelist", nil)
	responseRecorder := httptest.NewRecorder()
	httpHandler.ServeHTTP(responseRecorder, request)

	if responseRecorder.Code != http.StatusOK {
		testingInstance.Errorf("expected status 200, received %d", responseRecorder.Code)
	}

	responseBody := responseRecorder.Body.String()
	if strings.Contains(responseBody, "AA:BB:CC:DD:EE:FF") == false {
		testingInstance.Errorf("expected MAC address in body, received %q", responseBody)
	}
}

func TestCreateStaticDHCPHostAlsoAllowsMACAddress(testingInstance *testing.T) {
	fakeMACManager := &fakeMACWhitelistManager{}
	application := &Application{
		StaticDHCPManager:   &fakeStaticDHCPManager{},
		MACWhitelistManager: fakeMACManager,
	}
	httpHandler := newTestHTTPHandler(application, "")

	requestBody := strings.NewReader(`{"name":"laptop","macAddress":"aabbccddeeff","ipAddress":"192.168.1.100"}`)
	request := httptest.NewRequest("POST", "/api/static-dhcp-hosts", requestBody)
	responseRecorder := httptest.NewRecorder()
	httpHandler.ServeHTTP(responseRecorder, request)

	if responseRecorder.Code != http.StatusCreated {
		testingInstance.Errorf("expected status 201, received %d", responseRecorder.Code)
	}

	if fakeMACManager.addedMACAddress != "AA:BB:CC:DD:EE:FF" {
		testingInstance.Errorf("expected the normalized MAC address to be allowed, received %q", fakeMACManager.addedMACAddress)
	}

	if strings.Contains(responseRecorder.Body.String(), "warning") {
		testingInstance.Errorf("did not expect a warning, received %q", responseRecorder.Body.String())
	}
}

func TestCreateStaticDHCPHostReportsAllowListWarning(testingInstance *testing.T) {
	fakeMACManager := &fakeMACWhitelistManager{
		addError: errors.New("wireless reload failed"),
	}
	application := &Application{
		StaticDHCPManager:   &fakeStaticDHCPManager{},
		MACWhitelistManager: fakeMACManager,
	}
	httpHandler := newTestHTTPHandler(application, "")

	requestBody := strings.NewReader(`{"name":"laptop","macAddress":"AA:BB:CC:DD:EE:FF","ipAddress":"192.168.1.100"}`)
	request := httptest.NewRequest("POST", "/api/static-dhcp-hosts", requestBody)
	responseRecorder := httptest.NewRecorder()
	httpHandler.ServeHTTP(responseRecorder, request)

	if responseRecorder.Code != http.StatusCreated {
		testingInstance.Errorf("expected status 201 even when the allow list fails, received %d", responseRecorder.Code)
	}

	if strings.Contains(responseRecorder.Body.String(), "\"warning\"") == false {
		testingInstance.Errorf("expected a warning in the body, received %q", responseRecorder.Body.String())
	}
}

func TestCreateStaticDHCPHostSkipsAllowListOnFailure(testingInstance *testing.T) {
	fakeMACManager := &fakeMACWhitelistManager{}
	fakeDHCPManager := &fakeStaticDHCPManager{
		createError: errors.New("router refused the change"),
	}
	application := &Application{
		StaticDHCPManager:   fakeDHCPManager,
		MACWhitelistManager: fakeMACManager,
	}
	httpHandler := newTestHTTPHandler(application, "")

	requestBody := strings.NewReader(`{"name":"laptop","macAddress":"AA:BB:CC:DD:EE:FF","ipAddress":"192.168.1.100"}`)
	request := httptest.NewRequest("POST", "/api/static-dhcp-hosts", requestBody)
	responseRecorder := httptest.NewRecorder()
	httpHandler.ServeHTTP(responseRecorder, request)

	if responseRecorder.Code != http.StatusBadGateway {
		testingInstance.Errorf("expected status 502, received %d", responseRecorder.Code)
	}

	if fakeMACManager.addedMACAddress != "" {
		testingInstance.Errorf("expected the allow list not to be touched, received %q", fakeMACManager.addedMACAddress)
	}
}
