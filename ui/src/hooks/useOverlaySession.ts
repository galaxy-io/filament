import { useState } from "react";

export const useOverlaySession = (isOpen: boolean) => {
  const [session, setSession] = useState({ isOpen, key: isOpen ? 1 : 0 });
  if (session.isOpen !== isOpen) {
    setSession({ isOpen, key: isOpen ? session.key + 1 : session.key });
  }
  return session.key;
};
