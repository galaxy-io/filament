export enum LocalStorageKey {
  THEME = "THEME",
}

const LOCAL_STORAGE_KEY_MAP = {
  [LocalStorageKey.THEME]: "galaxy:theme",
} as const satisfies Record<LocalStorageKey, string>;

export const getLocalStorageKey = (key: LocalStorageKey) => LOCAL_STORAGE_KEY_MAP[key];
