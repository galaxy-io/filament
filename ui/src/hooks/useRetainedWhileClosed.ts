import { useRef } from "react";

export const useRetainedWhileClosed = <T>(value: T, isOpen: boolean): T => {
  const retainedRef = useRef(value);
  if (isOpen) retainedRef.current = value;
  return isOpen ? value : retainedRef.current;
};
