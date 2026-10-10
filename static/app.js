"use strict";

const AUTH_TOKEN_STORAGE_KEY = "openwrtController.authToken";
const API_BASE_URL_STORAGE_KEY = "openwrtController.apiBaseUrl";
const THEME_STORAGE_KEY = "openwrtController.theme";
const HEALTH_POLL_INTERVAL_MILLISECONDS = 30000;
const TOAST_DISPLAY_DURATION_MILLISECONDS = 5000;
const MAC_ADDRESS_PATTERN = /^([0-9A-Fa-f]{2}[:-]){5}[0-9A-Fa-f]{2}$/;
const IPV4_ADDRESS_PATTERN = /^((25[0-5]|2[0-4][0-9]|1[0-9][0-9]|[1-9]?[0-9])\.){3}(25[0-5]|2[0-4][0-9]|1[0-9][0-9]|[1-9]?[0-9])$/;
const ACTIVE_VIEW_NAMES = ["dhcp", "whitelist", "settings"];

const applicationState = {
  activeView: "dhcp",
  staticDhcpHosts: [],
  macWhitelistEntries: [],
  healthPollIdentifier: null,
};

document.addEventListener("DOMContentLoaded", handleDocumentReady);

function handleDocumentReady() {
  wireNavigationEvents();
  wireThemeToggleEvents();
  wirePrimaryActionEvents();
  wireModalEvents();
  wireStaticDhcpEvents();
  wireMacWhitelistEvents();
  wireSettingsEvents();
  restoreStoredSettings();
  registerServiceWorker();
  showView("dhcp");
  refreshHealthStatus();
  refreshStaticDhcpHosts();
  refreshMacWhitelistEntries();
  startHealthStatusPolling();
}

function registerServiceWorker() {
  const supportsServiceWorkers = "serviceWorker" in navigator;
  if (supportsServiceWorkers === false) {
    return;
  }

  const registrationPromise = navigator.serviceWorker.register("/service-worker.js");
  registrationPromise.catch(handleServiceWorkerRegistrationFailure);
}

function handleServiceWorkerRegistrationFailure(registrationError) {
  console.warn("Service worker registration failed:", registrationError);
}

function readStoredValue(storageKey, fallbackValue) {
  try {
    const storedValue = window.localStorage.getItem(storageKey);
    if (storedValue === null) {
      return fallbackValue;
    }
    return storedValue;
  } catch (storageError) {
    return fallbackValue;
  }
}

function writeStoredValue(storageKey, storageValue) {
  try {
    window.localStorage.setItem(storageKey, storageValue);
  } catch (storageError) {
    console.warn("Unable to persist the setting:", storageError);
  }
}

function buildApiUrl(requestPath) {
  const configuredBaseUrl = readStoredValue(API_BASE_URL_STORAGE_KEY, "");
  const trimmedBaseUrl = configuredBaseUrl.trim();
  const requestUrl = trimmedBaseUrl + requestPath;
  return requestUrl;
}

function buildRequestHeaders() {
  const requestHeaders = {
    "Content-Type": "application/json",
  };

  const authToken = readStoredValue(AUTH_TOKEN_STORAGE_KEY, "").trim();
  if (authToken !== "") {
    requestHeaders.Authorization = "Bearer " + authToken;
  }

  return requestHeaders;
}

async function performApiRequest(httpMethod, requestPath, requestBody) {
  const requestUrl = buildApiUrl(requestPath);
  const requestOptions = {
    method: httpMethod,
    headers: buildRequestHeaders(),
    cache: "no-store",
  };

  if (requestBody !== null) {
    requestOptions.body = JSON.stringify(requestBody);
  }

  const httpResponse = await fetch(requestUrl, requestOptions);
  const responseText = await httpResponse.text();

  let responsePayload = null;
  if (responseText !== "") {
    responsePayload = parseJsonSafely(responseText);
  }

  if (httpResponse.ok === false) {
    const errorMessage = extractErrorMessage(responsePayload, httpResponse.status);
    throw new Error(errorMessage);
  }

  return responsePayload;
}

function parseJsonSafely(responseText) {
  try {
    const parsedPayload = JSON.parse(responseText);
    return parsedPayload;
  } catch (parseError) {
    return null;
  }
}

function extractErrorMessage(responsePayload, statusCode) {
  if (responsePayload !== null) {
    if (typeof responsePayload === "object") {
      if (typeof responsePayload.error === "string") {
        return responsePayload.error;
      }
    }
  }

  return "Request failed with status " + statusCode;
}

function wireNavigationEvents() {
  const navigationButtons = document.querySelectorAll("[data-view-target]");
  for (let buttonIndex = 0; buttonIndex < navigationButtons.length; buttonIndex++) {
    const navigationButton = navigationButtons[buttonIndex];
    navigationButton.addEventListener("click", handleNavigationClick);
  }
}

function handleNavigationClick(clickEvent) {
  const clickedButton = clickEvent.currentTarget;
  const targetViewName = clickedButton.dataset.viewTarget;
  showView(targetViewName);
}

function showView(viewName) {
  applicationState.activeView = viewName;

  for (let viewIndex = 0; viewIndex < ACTIVE_VIEW_NAMES.length; viewIndex++) {
    const currentViewName = ACTIVE_VIEW_NAMES[viewIndex];
    const viewElement = document.getElementById("view-" + currentViewName);
    if (currentViewName === viewName) {
      viewElement.classList.remove("hidden");
    } else {
      viewElement.classList.add("hidden");
    }
  }

  updateNavigationState();
  updatePrimaryActionVisibility();
  window.scrollTo(0, 0);
}

function updateNavigationState() {
  const navigationButtons = document.querySelectorAll("[data-view-target]");
  for (let buttonIndex = 0; buttonIndex < navigationButtons.length; buttonIndex++) {
    const navigationButton = navigationButtons[buttonIndex];
    const targetViewName = navigationButton.dataset.viewTarget;
    const isActive = targetViewName === applicationState.activeView;
    if (isActive) {
      navigationButton.classList.add("is-active");
    } else {
      navigationButton.classList.remove("is-active");
    }
  }
}

function wireThemeToggleEvents() {
  const themeToggleButton = document.getElementById("theme-toggle-button");
  themeToggleButton.addEventListener("click", handleThemeToggleClick);
}

function handleThemeToggleClick() {
  const rootElement = document.documentElement;
  const isDarkTheme = rootElement.classList.contains("dark");
  if (isDarkTheme) {
    rootElement.classList.remove("dark");
    writeStoredValue(THEME_STORAGE_KEY, "light");
    updateThemeToggleLabel();
    return;
  }

  rootElement.classList.add("dark");
  writeStoredValue(THEME_STORAGE_KEY, "dark");
  updateThemeToggleLabel();
}

function updateThemeToggleLabel() {
  const themeToggleButton = document.getElementById("theme-toggle-button");
  const isDarkTheme = document.documentElement.classList.contains("dark");
  if (isDarkTheme) {
    themeToggleButton.textContent = "Light";
    themeToggleButton.setAttribute("aria-label", "Switch to light mode");
    return;
  }

  themeToggleButton.textContent = "Dark";
  themeToggleButton.setAttribute("aria-label", "Switch to dark mode");
}

function wirePrimaryActionEvents() {
  const primaryActionButton = document.getElementById("primary-action-button");
  primaryActionButton.addEventListener("click", handlePrimaryActionClick);
}

function handlePrimaryActionClick() {
  if (applicationState.activeView === "dhcp") {
    openStaticDhcpForm();
    return;
  }

  if (applicationState.activeView === "whitelist") {
    openMacWhitelistForm();
    return;
  }
}

function updatePrimaryActionVisibility() {
  const primaryActionButton = document.getElementById("primary-action-button");
  if (applicationState.activeView === "settings") {
    primaryActionButton.classList.add("hidden");
    return;
  }

  primaryActionButton.classList.remove("hidden");
}

function wireModalEvents() {
  document.addEventListener("keydown", handleDocumentKeyDown);
}

function handleDocumentKeyDown(keyEvent) {
  if (keyEvent.key !== "Escape") {
    return;
  }

  closeStaticDhcpForm();
  closeMacWhitelistForm();
}

function startHealthStatusPolling() {
  stopHealthStatusPolling();
  applicationState.healthPollIdentifier = window.setInterval(refreshHealthStatus, HEALTH_POLL_INTERVAL_MILLISECONDS);
}

function stopHealthStatusPolling() {
  if (applicationState.healthPollIdentifier !== null) {
    window.clearInterval(applicationState.healthPollIdentifier);
    applicationState.healthPollIdentifier = null;
  }
}

async function refreshHealthStatus() {
  try {
    await performApiRequest("GET", "/api/health", null);
    updateConnectionStatus(true, "Router connection OK");
    return;
  } catch (unexpectedError) {
    updateConnectionStatus(false, "Cannot reach the router API");
    return;
  }
}

function updateConnectionStatus(isConnected, statusMessage) {
  const statusIndicator = document.getElementById("connection-status");
  const statusText = document.getElementById("connection-status-text");
  statusText.textContent = statusMessage;

  statusIndicator.classList.remove("is-online");
  statusIndicator.classList.remove("is-offline");

  if (isConnected) {
    statusIndicator.classList.add("is-online");
    return;
  }

  statusIndicator.classList.add("is-offline");
}

function wireStaticDhcpEvents() {
  const refreshButton = document.getElementById("dhcp-refresh-button");
  const addButton = document.getElementById("dhcp-add-button");
  const formElement = document.getElementById("dhcp-form");
  const cancelButton = document.getElementById("dhcp-cancel-button");
  const closeButton = document.getElementById("dhcp-close-button");
  const backdropElement = document.getElementById("dhcp-modal-backdrop");

  refreshButton.addEventListener("click", handleStaticDhcpRefreshClick);
  addButton.addEventListener("click", handleStaticDhcpAddClick);
  formElement.addEventListener("submit", handleStaticDhcpFormSubmit);
  cancelButton.addEventListener("click", handleStaticDhcpCancelClick);
  closeButton.addEventListener("click", handleStaticDhcpCancelClick);
  backdropElement.addEventListener("click", handleStaticDhcpCancelClick);
}

function handleStaticDhcpRefreshClick() {
  refreshStaticDhcpHosts();
}

function handleStaticDhcpAddClick() {
  openStaticDhcpForm();
}

function handleStaticDhcpCancelClick() {
  closeStaticDhcpForm();
}

async function refreshStaticDhcpHosts() {
  const loadingElement = document.getElementById("dhcp-loading");
  loadingElement.classList.remove("hidden");

  try {
    const loadedHosts = await performApiRequest("GET", "/api/static-dhcp-hosts", null);
    if (Array.isArray(loadedHosts)) {
      applicationState.staticDhcpHosts = loadedHosts;
    } else {
      applicationState.staticDhcpHosts = [];
    }
    renderStaticDhcpHosts();
  } catch (unexpectedError) {
    showToast(unexpectedError.message, "error");
  } finally {
    loadingElement.classList.add("hidden");
  }
}

function renderStaticDhcpHosts() {
  const cardListElement = document.getElementById("dhcp-card-list");
  const tableBodyElement = document.getElementById("dhcp-table-body");
  const emptyStateElement = document.getElementById("dhcp-empty-state");

  clearChildElements(cardListElement);
  clearChildElements(tableBodyElement);

  const hostCount = applicationState.staticDhcpHosts.length;
  if (hostCount === 0) {
    emptyStateElement.classList.remove("hidden");
  } else {
    emptyStateElement.classList.add("hidden");
  }

  for (let hostIndex = 0; hostIndex < hostCount; hostIndex++) {
    const currentHost = applicationState.staticDhcpHosts[hostIndex];
    const cardElement = createStaticDhcpCard(currentHost);
    cardListElement.appendChild(cardElement);
    const rowElement = createStaticDhcpTableRow(currentHost);
    tableBodyElement.appendChild(rowElement);
  }
}

function createStaticDhcpCard(dhcpHost) {
  const cardElement = document.createElement("li");
  cardElement.className = "app-card";

  const headerElement = document.createElement("div");
  headerElement.className = "app-card-row--header";

  const nameElement = document.createElement("p");
  nameElement.className = "app-card-title";
  nameElement.textContent = dhcpHost.name;
  headerElement.appendChild(nameElement);

  const deleteButton = createDeleteButton(dhcpHost.macAddress, handleDeleteStaticDhcpHostClick);
  headerElement.appendChild(deleteButton);
  cardElement.appendChild(headerElement);

  const detailsElement = document.createElement("dl");
  detailsElement.className = "app-definition-list";
  appendDefinitionEntry(detailsElement, "MAC address", dhcpHost.macAddress);
  appendDefinitionEntry(detailsElement, "IP address", dhcpHost.ipAddress);
  cardElement.appendChild(detailsElement);

  return cardElement;
}

function createStaticDhcpTableRow(dhcpHost) {
  const rowElement = document.createElement("tr");
  rowElement.className = "app-table-row";

  appendTableCell(rowElement, dhcpHost.name, "app-table-cell font-medium");
  appendTableCell(rowElement, dhcpHost.macAddress, "app-table-cell app-mono");
  appendTableCell(rowElement, dhcpHost.ipAddress, "app-table-cell app-mono");

  const actionCellElement = document.createElement("td");
  actionCellElement.className = "app-table-cell app-table-cell--right";
  const deleteButton = createDeleteButton(dhcpHost.macAddress, handleDeleteStaticDhcpHostClick);
  actionCellElement.appendChild(deleteButton);
  rowElement.appendChild(actionCellElement);

  return rowElement;
}

function createDeleteButton(macAddress, clickHandler) {
  const deleteButton = document.createElement("button");
  deleteButton.type = "button";
  deleteButton.className = "app-button app-button--danger";
  deleteButton.textContent = "Delete";
  deleteButton.dataset.macAddress = macAddress;
  deleteButton.addEventListener("click", clickHandler);
  return deleteButton;
}

function handleDeleteStaticDhcpHostClick(clickEvent) {
  const clickedButton = clickEvent.currentTarget;
  const macAddress = clickedButton.dataset.macAddress;
  confirmAndDeleteStaticDhcpHost(macAddress);
}

async function confirmAndDeleteStaticDhcpHost(macAddress) {
  const shouldDelete = window.confirm("Delete the static DHCP reservation for " + macAddress + " ?");
  if (shouldDelete === false) {
    return;
  }

  try {
    await performApiRequest("DELETE", "/api/static-dhcp-hosts/" + macAddress, null);
    showToast("Static DHCP reservation deleted.", "success");
    await refreshStaticDhcpHosts();
  } catch (unexpectedError) {
    showToast(unexpectedError.message, "error");
  }
}

function openStaticDhcpForm() {
  resetStaticDhcpForm();
  const modalElement = document.getElementById("dhcp-modal");
  modalElement.classList.remove("hidden");
  const nameInput = document.getElementById("dhcp-name-input");
  nameInput.focus();
}

function closeStaticDhcpForm() {
  const modalElement = document.getElementById("dhcp-modal");
  modalElement.classList.add("hidden");
}

function resetStaticDhcpForm() {
  const formElement = document.getElementById("dhcp-form");
  formElement.reset();
  const errorElement = document.getElementById("dhcp-form-error");
  hideFormError(errorElement);
}

async function handleStaticDhcpFormSubmit(formSubmitEvent) {
  formSubmitEvent.preventDefault();

  const nameInput = document.getElementById("dhcp-name-input");
  const macInput = document.getElementById("dhcp-mac-input");
  const ipInput = document.getElementById("dhcp-ip-input");
  const errorElement = document.getElementById("dhcp-form-error");

  const hostName = nameInput.value.trim();
  const macAddress = macInput.value.trim();
  const ipAddress = ipInput.value.trim();

  const validationMessage = validateStaticDhcpInput(hostName, macAddress, ipAddress);
  if (validationMessage !== "") {
    showFormError(errorElement, validationMessage);
    return;
  }
  hideFormError(errorElement);

  const requestBody = {
    name: hostName,
    macAddress: macAddress,
    ipAddress: ipAddress,
  };

  try {
    await performApiRequest("POST", "/api/static-dhcp-hosts", requestBody);
    showToast("Reservation created.", "success");
    closeStaticDhcpForm();
    await refreshStaticDhcpHosts();
  } catch (unexpectedError) {
    showFormError(errorElement, unexpectedError.message);
  }
}

function validateStaticDhcpInput(hostName, macAddress, ipAddress) {
  if (hostName === "") {
    return "A hostname is required.";
  }

  if (MAC_ADDRESS_PATTERN.test(macAddress) === false) {
    return "Enter a valid MAC address such as AA:BB:CC:DD:EE:FF.";
  }

  if (IPV4_ADDRESS_PATTERN.test(ipAddress) === false) {
    return "Enter a valid IPv4 address such as 192.168.1.100.";
  }

  return "";
}

function wireMacWhitelistEvents() {
  const refreshButton = document.getElementById("whitelist-refresh-button");
  const addButton = document.getElementById("whitelist-add-button");
  const formElement = document.getElementById("whitelist-form");
  const cancelButton = document.getElementById("whitelist-cancel-button");
  const closeButton = document.getElementById("whitelist-close-button");
  const backdropElement = document.getElementById("whitelist-modal-backdrop");

  refreshButton.addEventListener("click", handleMacWhitelistRefreshClick);
  addButton.addEventListener("click", handleMacWhitelistAddClick);
  formElement.addEventListener("submit", handleMacWhitelistFormSubmit);
  cancelButton.addEventListener("click", handleMacWhitelistCancelClick);
  closeButton.addEventListener("click", handleMacWhitelistCancelClick);
  backdropElement.addEventListener("click", handleMacWhitelistCancelClick);
}

function handleMacWhitelistRefreshClick() {
  refreshMacWhitelistEntries();
}

function handleMacWhitelistAddClick() {
  openMacWhitelistForm();
}

function handleMacWhitelistCancelClick() {
  closeMacWhitelistForm();
}

async function refreshMacWhitelistEntries() {
  const loadingElement = document.getElementById("whitelist-loading");
  loadingElement.classList.remove("hidden");

  try {
    const loadedEntries = await performApiRequest("GET", "/api/mac-whitelist", null);
    if (Array.isArray(loadedEntries)) {
      applicationState.macWhitelistEntries = loadedEntries;
    } else {
      applicationState.macWhitelistEntries = [];
    }
    renderMacWhitelistEntries();
  } catch (unexpectedError) {
    showToast(unexpectedError.message, "error");
  } finally {
    loadingElement.classList.add("hidden");
  }
}

function renderMacWhitelistEntries() {
  const cardListElement = document.getElementById("whitelist-card-list");
  const tableBodyElement = document.getElementById("whitelist-table-body");
  const emptyStateElement = document.getElementById("whitelist-empty-state");

  clearChildElements(cardListElement);
  clearChildElements(tableBodyElement);

  const entryCount = applicationState.macWhitelistEntries.length;
  if (entryCount === 0) {
    emptyStateElement.classList.remove("hidden");
  } else {
    emptyStateElement.classList.add("hidden");
  }

  for (let entryIndex = 0; entryIndex < entryCount; entryIndex++) {
    const currentEntry = applicationState.macWhitelistEntries[entryIndex];
    const cardElement = createMacWhitelistCard(currentEntry);
    cardListElement.appendChild(cardElement);
    const rowElement = createMacWhitelistTableRow(currentEntry);
    tableBodyElement.appendChild(rowElement);
  }
}

function createMacWhitelistCard(whitelistEntry) {
  const cardElement = document.createElement("li");
  cardElement.className = "app-card app-card-row";

  const macElement = document.createElement("span");
  macElement.className = "app-mono";
  macElement.textContent = whitelistEntry.macAddress;
  cardElement.appendChild(macElement);

  const deleteButton = createDeleteButton(whitelistEntry.macAddress, handleRemoveMacWhitelistEntryClick);
  cardElement.appendChild(deleteButton);

  return cardElement;
}

function createMacWhitelistTableRow(whitelistEntry) {
  const rowElement = document.createElement("tr");
  rowElement.className = "app-table-row";

  appendTableCell(rowElement, whitelistEntry.macAddress, "app-table-cell app-mono");

  const actionCellElement = document.createElement("td");
  actionCellElement.className = "app-table-cell app-table-cell--right";
  const deleteButton = createDeleteButton(whitelistEntry.macAddress, handleRemoveMacWhitelistEntryClick);
  actionCellElement.appendChild(deleteButton);
  rowElement.appendChild(actionCellElement);

  return rowElement;
}

function handleRemoveMacWhitelistEntryClick(clickEvent) {
  const clickedButton = clickEvent.currentTarget;
  const macAddress = clickedButton.dataset.macAddress;
  confirmAndRemoveMacWhitelistEntry(macAddress);
}

async function confirmAndRemoveMacWhitelistEntry(macAddress) {
  const shouldRemove = window.confirm("Remove " + macAddress + " from the allow list ?");
  if (shouldRemove === false) {
    return;
  }

  try {
    await performApiRequest("DELETE", "/api/mac-whitelist/" + macAddress, null);
    showToast("MAC address removed from the allow list.", "success");
    await refreshMacWhitelistEntries();
  } catch (unexpectedError) {
    showToast(unexpectedError.message, "error");
  }
}

function openMacWhitelistForm() {
  resetMacWhitelistForm();
  const modalElement = document.getElementById("whitelist-modal");
  modalElement.classList.remove("hidden");
  const macInput = document.getElementById("whitelist-mac-input");
  macInput.focus();
}

function closeMacWhitelistForm() {
  const modalElement = document.getElementById("whitelist-modal");
  modalElement.classList.add("hidden");
}

function resetMacWhitelistForm() {
  const formElement = document.getElementById("whitelist-form");
  formElement.reset();
  const errorElement = document.getElementById("whitelist-form-error");
  hideFormError(errorElement);
}

async function handleMacWhitelistFormSubmit(formSubmitEvent) {
  formSubmitEvent.preventDefault();

  const macInput = document.getElementById("whitelist-mac-input");
  const errorElement = document.getElementById("whitelist-form-error");
  const macAddress = macInput.value.trim();

  if (MAC_ADDRESS_PATTERN.test(macAddress) === false) {
    showFormError(errorElement, "Enter a valid MAC address such as AA:BB:CC:DD:EE:FF.");
    return;
  }
  hideFormError(errorElement);

  const requestBody = {
    macAddress: macAddress,
  };

  try {
    await performApiRequest("POST", "/api/mac-whitelist", requestBody);
    showToast("MAC address allowed.", "success");
    closeMacWhitelistForm();
    await refreshMacWhitelistEntries();
  } catch (unexpectedError) {
    showFormError(errorElement, unexpectedError.message);
  }
}

function wireSettingsEvents() {
  const settingsFormElement = document.getElementById("settings-form");
  settingsFormElement.addEventListener("submit", handleSettingsFormSubmit);
}

function restoreStoredSettings() {
  const tokenInput = document.getElementById("settings-token-input");
  const apiBaseInput = document.getElementById("settings-api-base-input");

  tokenInput.value = readStoredValue(AUTH_TOKEN_STORAGE_KEY, "");
  apiBaseInput.value = readStoredValue(API_BASE_URL_STORAGE_KEY, "");

  updateThemeToggleLabel();
}

function handleSettingsFormSubmit(formSubmitEvent) {
  formSubmitEvent.preventDefault();

  const tokenInput = document.getElementById("settings-token-input");
  const apiBaseInput = document.getElementById("settings-api-base-input");

  writeStoredValue(AUTH_TOKEN_STORAGE_KEY, tokenInput.value.trim());
  writeStoredValue(API_BASE_URL_STORAGE_KEY, apiBaseInput.value.trim());

  showToast("Settings saved.", "success");
  refreshHealthStatus();
  refreshStaticDhcpHosts();
  refreshMacWhitelistEntries();
}

function showToast(toastMessage, toastKind) {
  const toastRegionElement = document.getElementById("toast-region");

  const toastElement = document.createElement("div");
  toastElement.className = "app-toast app-toast--" + toastKind;
  toastElement.setAttribute("role", "status");
  toastElement.textContent = toastMessage;
  toastRegionElement.appendChild(toastElement);

  window.setTimeout(removeToastElement, TOAST_DISPLAY_DURATION_MILLISECONDS, toastElement);
}

function removeToastElement(toastElement) {
  toastElement.remove();
}

function showFormError(errorElement, errorMessage) {
  errorElement.textContent = errorMessage;
  errorElement.classList.remove("hidden");
}

function hideFormError(errorElement) {
  errorElement.textContent = "";
  errorElement.classList.add("hidden");
}

function clearChildElements(parentElement) {
  while (parentElement.firstChild !== null) {
    parentElement.removeChild(parentElement.firstChild);
  }
}

function appendTableCell(rowElement, cellText, cellClassName) {
  const cellElement = document.createElement("td");
  cellElement.className = cellClassName;
  cellElement.textContent = cellText;
  rowElement.appendChild(cellElement);
}

function appendDefinitionEntry(definitionListElement, termText, descriptionText) {
  const entryElement = document.createElement("div");
  entryElement.className = "app-card-row";

  const termElement = document.createElement("dt");
  termElement.className = "app-definition-term";
  termElement.textContent = termText;
  entryElement.appendChild(termElement);

  const descriptionElement = document.createElement("dd");
  descriptionElement.className = "app-mono";
  descriptionElement.textContent = descriptionText;
  entryElement.appendChild(descriptionElement);

  definitionListElement.appendChild(entryElement);
}








