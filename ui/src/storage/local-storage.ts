export enum LocalStorageKey {
  THEME = "THEME",
}

const LOCAL_STORAGE_KEYS: Record<LocalStorageKey, string> = {
  // Keep in sync with the theme flash-prevention script in index.html.
  [LocalStorageKey.THEME]: "filament:theme",
};

export const getLocalStorageKey = (key: LocalStorageKey): string => {
  return LOCAL_STORAGE_KEYS[key];
};
