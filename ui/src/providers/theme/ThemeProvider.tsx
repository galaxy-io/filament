import type { PropsWithChildren } from "react";

import { GalaxyTheme } from "@galaxy-io/dls/theme/enums";
import GalaxyProvider from "@galaxy-io/dls/theme/GalaxyProvider";

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
    <GalaxyProvider
      theme={storedTheme}
      storageKey={storageKey}
      storage={{
        get: () => storedTheme,
        set: (_, theme) => setStoredTheme(theme),
      }}
    >
      {children}
    </GalaxyProvider>
  );
};

export default ThemeProvider;
