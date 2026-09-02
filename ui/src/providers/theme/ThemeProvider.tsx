import type { PropsWithChildren } from "react";

import { GalaxyThemeProvider } from "@galaxy-io/dls/theme/GalaxyTheme";
import { GalaxyTheme } from "@galaxy-io/dls/theme/types";

import useLocalStorage from "@/hooks/useLocalStorage";

import { getLocalStorageKey, LocalStorageKey } from "@/storage/local-storage";

const ThemeProvider = ({ children }: PropsWithChildren) => {
  const storageKey = getLocalStorageKey(LocalStorageKey.THEME);
  const [storedTheme, setStoredTheme] = useLocalStorage(
    LocalStorageKey.THEME,
    GalaxyTheme.SYSTEM,
    GalaxyTheme,
  );

  return (
    <GalaxyThemeProvider
      initialTheme={storedTheme}
      storageKey={storageKey}
      storage={{
        get: () => storedTheme,
        set: (_, theme) => setStoredTheme(theme),
      }}
    >
      {children}
    </GalaxyThemeProvider>
  );
};

export default ThemeProvider;
