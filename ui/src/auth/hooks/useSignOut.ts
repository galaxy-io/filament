import { useCallback } from "react";

import { useAuth } from "react-oidc-context";

import { useLogoutMutation } from "@/api/mutations/auth";

export const useSignOut = () => {
  const { signoutRedirect } = useAuth();
  const { mutateAsync: logout } = useLogoutMutation();

  return useCallback(async () => {
    await logout({}).catch(() => undefined);
    await signoutRedirect();
  }, [logout, signoutRedirect]);
};
