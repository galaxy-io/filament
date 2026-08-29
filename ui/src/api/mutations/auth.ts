import { type UseMutationOptions, useMutation } from "@connectrpc/connect-query";
import { useQueryClient } from "@tanstack/react-query";

import { AuthService } from "@/gen/auth/v1/service_pb";

import { createListMembersQueryKey, createListServiceAccountsQueryKey } from "@/api/queries/auth";

export const useLoginMutation = (
  options: UseMutationOptions<
    typeof AuthService.method.login.input,
    typeof AuthService.method.login.output
  > = {},
) => {
  return useMutation<typeof AuthService.method.login.input, typeof AuthService.method.login.output>(
    AuthService.method.login,
    options,
  );
};

export const useRegisterMutation = (
  options: UseMutationOptions<
    typeof AuthService.method.register.input,
    typeof AuthService.method.register.output
  > = {},
) => {
  return useMutation<
    typeof AuthService.method.register.input,
    typeof AuthService.method.register.output
  >(AuthService.method.register, options);
};

export const useAcceptInviteMutation = (
  options: UseMutationOptions<
    typeof AuthService.method.acceptInvite.input,
    typeof AuthService.method.acceptInvite.output
  > = {},
) => {
  return useMutation<
    typeof AuthService.method.acceptInvite.input,
    typeof AuthService.method.acceptInvite.output
  >(AuthService.method.acceptInvite, options);
};

export const useInviteMemberMutation = (
  options: UseMutationOptions<
    typeof AuthService.method.inviteMember.input,
    typeof AuthService.method.inviteMember.output
  > = {},
) => {
  const queryClient = useQueryClient();
  return useMutation<
    typeof AuthService.method.inviteMember.input,
    typeof AuthService.method.inviteMember.output
  >(AuthService.method.inviteMember, {
    ...options,
    onSettled: (...args) => {
      void queryClient.invalidateQueries({ queryKey: createListMembersQueryKey() });
      return options.onSettled?.(...args);
    },
  });
};

export const useSetMemberRoleMutation = (
  options: UseMutationOptions<
    typeof AuthService.method.setMemberRole.input,
    typeof AuthService.method.setMemberRole.output
  > = {},
) => {
  const queryClient = useQueryClient();
  return useMutation<
    typeof AuthService.method.setMemberRole.input,
    typeof AuthService.method.setMemberRole.output
  >(AuthService.method.setMemberRole, {
    ...options,
    onSettled: (...args) => {
      void queryClient.invalidateQueries({ queryKey: createListMembersQueryKey() });
      return options.onSettled?.(...args);
    },
  });
};

export const useRemoveMemberMutation = (
  options: UseMutationOptions<
    typeof AuthService.method.removeMember.input,
    typeof AuthService.method.removeMember.output
  > = {},
) => {
  const queryClient = useQueryClient();
  return useMutation<
    typeof AuthService.method.removeMember.input,
    typeof AuthService.method.removeMember.output
  >(AuthService.method.removeMember, {
    ...options,
    onSettled: (...args) => {
      void queryClient.invalidateQueries({ queryKey: createListMembersQueryKey() });
      return options.onSettled?.(...args);
    },
  });
};

export const useCreateServiceAccountMutation = (
  options: UseMutationOptions<
    typeof AuthService.method.createServiceAccount.input,
    typeof AuthService.method.createServiceAccount.output
  > = {},
) => {
  const queryClient = useQueryClient();
  return useMutation<
    typeof AuthService.method.createServiceAccount.input,
    typeof AuthService.method.createServiceAccount.output
  >(AuthService.method.createServiceAccount, {
    ...options,
    onSettled: (...args) => {
      void queryClient.invalidateQueries({ queryKey: createListServiceAccountsQueryKey() });
      return options.onSettled?.(...args);
    },
  });
};

export const useRotateServiceAccountSecretMutation = (
  options: UseMutationOptions<
    typeof AuthService.method.rotateServiceAccountSecret.input,
    typeof AuthService.method.rotateServiceAccountSecret.output
  > = {},
) => {
  return useMutation<
    typeof AuthService.method.rotateServiceAccountSecret.input,
    typeof AuthService.method.rotateServiceAccountSecret.output
  >(AuthService.method.rotateServiceAccountSecret, options);
};

export const useRemoveServiceAccountMutation = (
  options: UseMutationOptions<
    typeof AuthService.method.removeServiceAccount.input,
    typeof AuthService.method.removeServiceAccount.output
  > = {},
) => {
  const queryClient = useQueryClient();
  return useMutation<
    typeof AuthService.method.removeServiceAccount.input,
    typeof AuthService.method.removeServiceAccount.output
  >(AuthService.method.removeServiceAccount, {
    ...options,
    onSettled: (...args) => {
      void queryClient.invalidateQueries({ queryKey: createListServiceAccountsQueryKey() });
      return options.onSettled?.(...args);
    },
  });
};
