import { useCallback } from "react";

import { create } from "@bufbuild/protobuf";
import { useQueryClient } from "@tanstack/react-query";
import { useNavigate } from "@tanstack/react-router";

import { LogoutRequestSchema } from "@/gen/auth/v1/session_pb";

import { useLogoutMutation } from "@/api/queries/auth";

export const useSignOut = () => {
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const { mutateAsync: logout } = useLogoutMutation();

  return useCallback(async () => {
    await logout(create(LogoutRequestSchema, {})).catch(() => undefined);
    await navigate({ to: "/login", replace: true });
    queryClient.clear();
  }, [logout, navigate, queryClient]);
};
