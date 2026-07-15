import { type PropsWithChildren, useCallback, useState } from "react";

import { GalaxyTheme } from "@galaxy-io/dls/theme/constants";
import { GalaxyThemeProvider } from "@galaxy-io/dls/theme/GalaxyTheme";

import { getLocalStorageKey, LocalStorageKey } from "@/storage/local-storage";

const STORAGE_KEY = getLocalStorageKey(LocalStorageKey.THEME);

const isGalaxyTheme = (value: string | null): value is GalaxyTheme => {
  return value !== null && Object.values(GalaxyTheme).includes(value as GalaxyTheme);
};

const readStoredTheme = (): GalaxyTheme => {
  try {
    const stored = window.localStorage.getItem(STORAGE_KEY);
    return isGalaxyTheme(stored) ? stored : GalaxyTheme.SYSTEM;
  } catch {
    return GalaxyTheme.SYSTEM;
  }
};

const ThemeProvider = ({ children }: PropsWithChildren) => {
  const [storedTheme, setStoredTheme] = useState<GalaxyTheme>(readStoredTheme);

  const handleSetTheme = useCallback((_: string, theme: GalaxyTheme) => {
    setStoredTheme(theme);
    try {
      window.localStorage.setItem(STORAGE_KEY, theme);
    } catch {
      // localStorage unavailable - theme resets on reload
    }
  }, []);

  return (
    <GalaxyThemeProvider
      initialTheme={storedTheme}
      storageKey={STORAGE_KEY}
      storage={{
        get: () => storedTheme,
        set: handleSetTheme,
      }}
    >
      {children}
    </GalaxyThemeProvider>
  );
};

export default ThemeProvider;
