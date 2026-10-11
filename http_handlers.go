// HTTP handlers: the JSON API and its responses.
//
// A "handler" is a function that receives an HTTP request and writes a response.
// This file contains one handler per API route, plus small helpers that encode
// JSON and that translate service errors into HTTP status codes.
//
// Python equivalent: this is close to the functions you write with Flask or
// FastAPI, one per route. Here they are methods of the Application struct and
// there is no framework; the standard library does the work.

package main

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
)

// HealthResponse is the payload returned by the health endpoint.
type HealthResponse struct {
	Status string `json:"status"`
}

// ErrorResponse is the payload returned whenever a request cannot be fulfilled.
type ErrorResponse struct {
	Error string `json:"error"`
}

// CreatedStaticDHCPHostResponse is the payload returned after creating or updating
// a static DHCP reservation. Warning is filled in when the reservation was saved but
// its MAC address could not also be added to the wireless allow list.
type CreatedStaticDHCPHostResponse struct {
	Name       string `json:"name"`
	MACAddress string `json:"macAddress"`
	IPAddress  string `json:"ipAddress"`
	Warning    string `json:"warning,omitempty"`
}

// Application groups every manager used by the HTTP handlers.
type Application struct {
	StaticDHCPManager   StaticDHCPManager
	MACWhitelistManager MACWhitelistManager
}

// writeJSONResponse writes a JSON body plus the HTTP status code. The value is
// encoded with encoding/json, which is the standard library equivalent of
// json.dumps in Python.
func writeJSONResponse(responseWriter http.ResponseWriter, statusCode int, responsePayload interface{}) {
	responseWriter.Header().Set("Content-Type", "application/json")
	responseWriter.WriteHeader(statusCode)

	jsonEncoder := json.NewEncoder(responseWriter)
	encodeError := jsonEncoder.Encode(responsePayload)
	if encodeError != nil {
		log.Printf("failed to encode JSON response: %v", encodeError)
	}
}

// writeErrorResponse returns a small JSON object of the form {"error": "..."}.
func writeErrorResponse(responseWriter http.ResponseWriter, statusCode int, errorMessage string) {
	errorPayload := ErrorResponse{Error: errorMessage}
	writeJSONResponse(responseWriter, statusCode, errorPayload)
}

// serviceErrorStatusCode chooses the HTTP status code for a service error:
// 400 for invalid input, 404 when something was not found, and 502 when the
// router could not be reached or refused to apply the change.
func serviceErrorStatusCode(serviceError error) int {
	var validationError *ValidationError
	validationErrorDetected := errors.As(serviceError, &validationError)
	if validationErrorDetected {
		return http.StatusBadRequest
	}

	notFoundDetected := errors.Is(serviceError, StaticDHCPHostNotFoundError)
	if notFoundDetected {
		return http.StatusNotFound
	}

	notFoundDetected = errors.Is(serviceError, MACWhitelistEntryNotFoundError)
	if notFoundDetected {
		return http.StatusNotFound
	}

	conflictDetected := errors.Is(serviceError, StaticDHCPHostConflictError)
	if conflictDetected {
		return http.StatusConflict
	}

	return http.StatusBadGateway
}

// handleHealthRequest answers GET /api/health with {"status":"ok"}. The frontend
// uses it to show whether the API is reachable.
func (application *Application) handleHealthRequest(responseWriter http.ResponseWriter, request *http.Request) {
	healthResponse := HealthResponse{Status: "ok"}
	writeJSONResponse(responseWriter, http.StatusOK, healthResponse)
}

// handleListStaticDHCPHosts answers GET /api/static-dhcp-hosts with a JSON array.
func (application *Application) handleListStaticDHCPHosts(responseWriter http.ResponseWriter, request *http.Request) {
	staticDHCPHosts, serviceError := application.StaticDHCPManager.ListStaticDHCPHosts()
	if serviceError != nil {
		statusCode := serviceErrorStatusCode(serviceError)
		writeErrorResponse(responseWriter, statusCode, serviceError.Error())
		return
	}

	if staticDHCPHosts == nil {
		staticDHCPHosts = make([]StaticDHCPHost, 0)
	}

	writeJSONResponse(responseWriter, http.StatusOK, staticDHCPHosts)
}

// handleCreateStaticDHCPHost answers POST /api/static-dhcp-hosts. It decodes the
// JSON body into a StaticDHCPHost and returns HTTP 201 when the reservation was
// created.
//
// Creating a reservation also allows its MAC address on Wi-Fi, because the two
// features usually go together. That second step is best effort: when it fails the
// reservation is still created and the problem is reported in the "warning" field.
func (application *Application) handleCreateStaticDHCPHost(responseWriter http.ResponseWriter, request *http.Request) {
	var requestedHost StaticDHCPHost
	decodeError := json.NewDecoder(request.Body).Decode(&requestedHost)
	if decodeError != nil {
		writeErrorResponse(responseWriter, http.StatusBadRequest, "invalid JSON request body")
		return
	}

	canonicalMACAddress, normalizeError := NormalizeMACAddress(requestedHost.MACAddress)
	if normalizeError != nil {
		statusCode := serviceErrorStatusCode(normalizeError)
		writeErrorResponse(responseWriter, statusCode, normalizeError.Error())
		return
	}

	createError := application.StaticDHCPManager.CreateStaticDHCPHost(requestedHost)
	if createError != nil {
		statusCode := serviceErrorStatusCode(createError)
		writeErrorResponse(responseWriter, statusCode, createError.Error())
		return
	}

	createdHostResponse := CreatedStaticDHCPHostResponse{
		Name:       requestedHost.Name,
		MACAddress: canonicalMACAddress,
		IPAddress:  requestedHost.IPAddress,
	}

	allowListError := application.MACWhitelistManager.AddMACWhitelistEntry(canonicalMACAddress)
	if allowListError != nil {
		createdHostResponse.Warning = "the reservation was created but the MAC address could not be added to the Wi-Fi allow list: " + allowListError.Error()
	}

	writeJSONResponse(responseWriter, http.StatusCreated, createdHostResponse)
}

// handleUpdateStaticDHCPHost answers PUT /api/static-dhcp-hosts/{macAddress}. The
// value in the URL is the MAC address as it is today; the JSON body carries the
// desired new state, which may include a different MAC address.
func (application *Application) handleUpdateStaticDHCPHost(responseWriter http.ResponseWriter, request *http.Request) {
	currentMACAddress := request.PathValue("macAddress")

	var requestedHost StaticDHCPHost
	decodeError := json.NewDecoder(request.Body).Decode(&requestedHost)
	if decodeError != nil {
		writeErrorResponse(responseWriter, http.StatusBadRequest, "invalid JSON request body")
		return
	}

	canonicalCurrentMACAddress, normalizeCurrentError := NormalizeMACAddress(currentMACAddress)
	if normalizeCurrentError != nil {
		statusCode := serviceErrorStatusCode(normalizeCurrentError)
		writeErrorResponse(responseWriter, statusCode, normalizeCurrentError.Error())
		return
	}

	canonicalUpdatedMACAddress, normalizeUpdatedError := NormalizeMACAddress(requestedHost.MACAddress)
	if normalizeUpdatedError != nil {
		statusCode := serviceErrorStatusCode(normalizeUpdatedError)
		writeErrorResponse(responseWriter, statusCode, normalizeUpdatedError.Error())
		return
	}

	updateError := application.StaticDHCPManager.UpdateStaticDHCPHost(canonicalCurrentMACAddress, requestedHost)
	if updateError != nil {
		statusCode := serviceErrorStatusCode(updateError)
		writeErrorResponse(responseWriter, statusCode, updateError.Error())
		return
	}

	updatedHostResponse := CreatedStaticDHCPHostResponse{
		Name:       requestedHost.Name,
		MACAddress: canonicalUpdatedMACAddress,
		IPAddress:  requestedHost.IPAddress,
	}

	allowListWarning := application.synchronizeAllowListAfterUpdate(canonicalCurrentMACAddress, canonicalUpdatedMACAddress)
	if allowListWarning != "" {
		updatedHostResponse.Warning = allowListWarning
	}

	writeJSONResponse(responseWriter, http.StatusOK, updatedHostResponse)
}

// synchronizeAllowListAfterUpdate makes sure the new MAC address is allowed and, when
// the MAC address changed, that the old one is removed if no other reservation still
// uses it. It returns a warning message when something could not be synchronized.
func (application *Application) synchronizeAllowListAfterUpdate(currentMACAddress string, updatedMACAddress string) string {
	allowListError := application.MACWhitelistManager.AddMACWhitelistEntry(updatedMACAddress)
	if allowListError != nil {
		return "the reservation was updated but the new MAC address could not be added to the Wi-Fi allow list: " + allowListError.Error()
	}

	macAddressChanged := macAddressesEqual(currentMACAddress, updatedMACAddress) == false
	if macAddressChanged == false {
		return ""
	}

	otherHostUsesCurrentMACAddress, lookupError := application.anotherHostUsesMACAddress(updatedMACAddress, currentMACAddress)
	if lookupError != nil {
		return "the reservation was updated but the old MAC address could not be checked: " + lookupError.Error()
	}

	if otherHostUsesCurrentMACAddress {
		return ""
	}

	removeError := application.MACWhitelistManager.RemoveMACWhitelistEntry(currentMACAddress)
	if removeError != nil {
		notFoundDetected := errors.Is(removeError, MACWhitelistEntryNotFoundError)
		if notFoundDetected == false {
			return "the reservation was updated but the old MAC address could not be removed from the Wi-Fi allow list: " + removeError.Error()
		}
	}

	return ""
}

// anotherHostUsesMACAddress reports whether any reservation other than
// excludedMACAddress still uses candidateMACAddress.
func (application *Application) anotherHostUsesMACAddress(excludedMACAddress string, candidateMACAddress string) (bool, error) {
	staticDHCPHosts, listError := application.StaticDHCPManager.ListStaticDHCPHosts()
	if listError != nil {
		return false, listError
	}

	for hostIndex := 0; hostIndex < len(staticDHCPHosts); hostIndex++ {
		currentHost := staticDHCPHosts[hostIndex]
		if macAddressesEqual(currentHost.MACAddress, excludedMACAddress) {
			continue
		}

		if macAddressesEqual(currentHost.MACAddress, candidateMACAddress) {
			return true, nil
		}
	}

	return false, nil
}

// handleDeleteStaticDHCPHost answers DELETE /api/static-dhcp-hosts/{macAddress}.
// request.PathValue reads the part of the URL inside the curly braces.
func (application *Application) handleDeleteStaticDHCPHost(responseWriter http.ResponseWriter, request *http.Request) {
	macAddress := request.PathValue("macAddress")

	deleteError := application.StaticDHCPManager.DeleteStaticDHCPHost(macAddress)
	if deleteError != nil {
		statusCode := serviceErrorStatusCode(deleteError)
		writeErrorResponse(responseWriter, statusCode, deleteError.Error())
		return
	}

	responseWriter.WriteHeader(http.StatusNoContent)
}

// handleListMACWhitelistEntries answers GET /api/mac-whitelist with a JSON array.
func (application *Application) handleListMACWhitelistEntries(responseWriter http.ResponseWriter, request *http.Request) {
	macWhitelistEntries, serviceError := application.MACWhitelistManager.ListMACWhitelistEntries()
	if serviceError != nil {
		statusCode := serviceErrorStatusCode(serviceError)
		writeErrorResponse(responseWriter, statusCode, serviceError.Error())
		return
	}

	if macWhitelistEntries == nil {
		macWhitelistEntries = make([]MACWhitelistEntry, 0)
	}

	writeJSONResponse(responseWriter, http.StatusOK, macWhitelistEntries)
}

// handleAddMACWhitelistEntry answers POST /api/mac-whitelist.
func (application *Application) handleAddMACWhitelistEntry(responseWriter http.ResponseWriter, request *http.Request) {
	var requestedEntry MACWhitelistEntry
	decodeError := json.NewDecoder(request.Body).Decode(&requestedEntry)
	if decodeError != nil {
		writeErrorResponse(responseWriter, http.StatusBadRequest, "invalid JSON request body")
		return
	}

	addError := application.MACWhitelistManager.AddMACWhitelistEntry(requestedEntry.MACAddress)
	if addError != nil {
		statusCode := serviceErrorStatusCode(addError)
		writeErrorResponse(responseWriter, statusCode, addError.Error())
		return
	}

	writeJSONResponse(responseWriter, http.StatusCreated, requestedEntry)
}

// handleRemoveMACWhitelistEntry answers DELETE /api/mac-whitelist/{macAddress}.
func (application *Application) handleRemoveMACWhitelistEntry(responseWriter http.ResponseWriter, request *http.Request) {
	macAddress := request.PathValue("macAddress")

	removeError := application.MACWhitelistManager.RemoveMACWhitelistEntry(macAddress)
	if removeError != nil {
		statusCode := serviceErrorStatusCode(removeError)
		writeErrorResponse(responseWriter, statusCode, removeError.Error())
		return
	}

	responseWriter.WriteHeader(http.StatusNoContent)
}
