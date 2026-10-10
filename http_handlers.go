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

func writeJSONResponse(responseWriter http.ResponseWriter, statusCode int, responsePayload interface{}) {
	responseWriter.Header().Set("Content-Type", "application/json")
	responseWriter.WriteHeader(statusCode)

	jsonEncoder := json.NewEncoder(responseWriter)
	encodeError := jsonEncoder.Encode(responsePayload)
	if encodeError != nil {
		log.Printf("failed to encode JSON response: %v", encodeError)
	}
}

func writeErrorResponse(responseWriter http.ResponseWriter, statusCode int, errorMessage string) {
	errorPayload := ErrorResponse{Error: errorMessage}
	writeJSONResponse(responseWriter, statusCode, errorPayload)
}

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

func (application *Application) handleHealthRequest(responseWriter http.ResponseWriter, request *http.Request) {
	healthResponse := HealthResponse{Status: "ok"}
	writeJSONResponse(responseWriter, http.StatusOK, healthResponse)
}

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
