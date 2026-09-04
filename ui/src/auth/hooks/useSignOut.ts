import { useCallback } from "react";

import { useQueryClient } from "@tanstack/react-query";
import { useNavigate } from "@tanstack/react-router";

import { useLogoutMutation } from "@/api/mutations/auth";

export const useSignOut = () => {
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const { mutateAsync: logout } = useLogoutMutation();

  return useCallback(async () => {
    await logout({}).catch(() => undefined);
    await navigate({ to: "/login", replace: true });
    queryClient.clear();
  }, [logout, navigate, queryClient]);
};
