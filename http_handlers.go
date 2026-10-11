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
func (application *Application) handleCreateStaticDHCPHost(responseWriter http.ResponseWriter, request *http.Request) {
	var requestedHost StaticDHCPHost
	decodeError := json.NewDecoder(request.Body).Decode(&requestedHost)
	if decodeError != nil {
		writeErrorResponse(responseWriter, http.StatusBadRequest, "invalid JSON request body")
		return
	}

	createError := application.StaticDHCPManager.CreateStaticDHCPHost(requestedHost)
	if createError != nil {
		statusCode := serviceErrorStatusCode(createError)
		writeErrorResponse(responseWriter, statusCode, createError.Error())
		return
	}

	writeJSONResponse(responseWriter, http.StatusCreated, requestedHost)
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
