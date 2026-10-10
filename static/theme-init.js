"use strict";

// Applies the stored (or system) theme before the first paint to avoid a flash.
function applyInitialTheme() {
  const themeStorageKey = "openwrtController.theme";
  const rootElement = document.documentElement;
  let resolvedTheme = "light";

  try {
    const storedTheme = window.localStorage.getItem(themeStorageKey);
    if (storedTheme === "dark") {
      resolvedTheme = "dark";
    } else if (storedTheme === "light") {
      resolvedTheme = "light";
    } else {
      const prefersDarkQuery = window.matchMedia("(prefers-color-scheme: dark)");
      if (prefersDarkQuery.matches) {
        resolvedTheme = "dark";
      }
    }
  } catch (storageError) {
    resolvedTheme = "light";
  }

  if (resolvedTheme === "dark") {
    rootElement.classList.add("dark");
  } else {
    rootElement.classList.remove("dark");
  }
}

applyInitialTheme();
