import {
  type Dispatch,
  type SetStateAction,
  useCallback,
  useEffect,
  useMemo,
  useState,
} from "react";

import { getLocalStorageKey, type LocalStorageKey } from "@/storage/local-storage";

type StringEnum = Record<string, string>;
type EnumValue<TEnum extends StringEnum> = TEnum[keyof TEnum];

const readLocalStorage = (storageKey: string): string | null => {
  try {
    return window.localStorage.getItem(storageKey);
  } catch {
    return null;
  }
};

const writeLocalStorage = (storageKey: string, value: string) => {
  try {
    window.localStorage.setItem(storageKey, value);
  } catch {
    return;
  }
};

const useLocalStorage = <TEnum extends StringEnum>(
  key: LocalStorageKey,
  initialValue: EnumValue<TEnum>,
  enumType: TEnum,
): [EnumValue<TEnum>, Dispatch<SetStateAction<EnumValue<TEnum>>>] => {
  const storageKey = getLocalStorageKey(key);
  const validValues = useMemo(() => Object.values(enumType) as EnumValue<TEnum>[], [enumType]);

  const isValidValue = useCallback(
    (value: unknown): value is EnumValue<TEnum> =>
      typeof value === "string" && validValues.includes(value as EnumValue<TEnum>),
    [validValues],
  );

  const [value, setValue] = useState<EnumValue<TEnum>>(() => {
    const storedValue = readLocalStorage(storageKey);
    return isValidValue(storedValue) ? storedValue : initialValue;
  });

  useEffect(() => {
    if (!isValidValue(readLocalStorage(storageKey))) {
      writeLocalStorage(storageKey, value);
    }
  }, [isValidValue, storageKey, value]);

  const setStoredValue = useCallback<Dispatch<SetStateAction<EnumValue<TEnum>>>>(
    (nextValue) => {
      const resolvedValue = typeof nextValue === "function" ? nextValue(value) : nextValue;

      writeLocalStorage(storageKey, resolvedValue);
      setValue(resolvedValue);
    },
    [storageKey, value],
  );

  return [value, setStoredValue];
};

export default useLocalStorage;
