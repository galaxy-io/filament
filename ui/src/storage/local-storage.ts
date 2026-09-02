import { GALAXY_THEME_STORAGE_KEY } from "@galaxy-io/dls/theme/constants";

export enum LocalStorageKey {
  THEME = "THEME",
}

const LOCAL_STORAGE_KEY_MAP = {
  [LocalStorageKey.THEME]: GALAXY_THEME_STORAGE_KEY,
} as const satisfies Record<LocalStorageKey, string>;

export const getLocalStorageKey = (key: LocalStorageKey) => LOCAL_STORAGE_KEY_MAP[key];
