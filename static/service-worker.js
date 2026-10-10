"use strict";

const APPLICATION_SHELL_CACHE_NAME = "openwrt-controller-shell-v1";
const APPLICATION_SHELL_URLS = [
  "/",
  "/index.html",
  "/app.js",
  "/theme-init.js",
  "/tailwind-config.js",
  "/manifest.json",
  "/icon.svg",
];

self.addEventListener("install", handleInstallEvent);
self.addEventListener("activate", handleActivateEvent);
self.addEventListener("fetch", handleFetchEvent);

function handleInstallEvent(installEvent) {
  installEvent.waitUntil(precacheApplicationShell());
}

async function precacheApplicationShell() {
  const cacheInstance = await caches.open(APPLICATION_SHELL_CACHE_NAME);
  await cacheInstance.addAll(APPLICATION_SHELL_URLS);
  await self.skipWaiting();
}

function handleActivateEvent(activateEvent) {
  activateEvent.waitUntil(removeObsoleteCaches());
}

async function removeObsoleteCaches() {
  const knownCacheNames = await caches.keys();
  for (let cacheIndex = 0; cacheIndex < knownCacheNames.length; cacheIndex++) {
    const currentCacheName = knownCacheNames[cacheIndex];
    if (currentCacheName !== APPLICATION_SHELL_CACHE_NAME) {
      await caches.delete(currentCacheName);
    }
  }
  await self.clients.claim();
}

function handleFetchEvent(fetchEvent) {
  if (fetchEvent.request.method !== "GET") {
    return;
  }

  const requestUrl = new URL(fetchEvent.request.url);
  if (requestUrl.origin !== self.location.origin) {
    return;
  }

  const isApiRequest = requestUrl.pathname.startsWith("/api/");
  if (isApiRequest) {
    return;
  }

  fetchEvent.respondWith(serveApplicationShellResource(fetchEvent.request));
}

async function serveApplicationShellResource(request) {
  const cachedResponse = await caches.match(request);
  if (cachedResponse !== undefined) {
    return cachedResponse;
  }

  try {
    const networkResponse = await fetch(request);
    return networkResponse;
  } catch (networkError) {
    const fallbackResponse = await caches.match("/index.html");
    if (fallbackResponse !== undefined) {
      return fallbackResponse;
    }
    throw networkError;
  }
}
