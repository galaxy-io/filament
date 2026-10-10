import type { Transport } from "@connectrpc/connect";
import {
  createConnectQueryKey,
  createQueryOptions,
  type UseMutationOptions,
  type UseQueryOptions,
  useMutation,
  useQuery,
  useTransport,
} from "@connectrpc/connect-query";
import { useQueryClient } from "@tanstack/react-query";

import type { ListMembersRequest, ListMembersResponse } from "@/gen/auth/v1/members_pb";
import type {
  ListServiceAccountsRequest,
  ListServiceAccountsResponse,
} from "@/gen/auth/v1/service_accounts_pb";
import { AuthService } from "@/gen/auth/v1/service_pb";
import type { GetSessionResponse } from "@/gen/auth/v1/session_pb";

import { PROBE_QUERY_OPTIONS } from "@/api/queries/constants";

export const createListMembersQueryKey = (input?: ListMembersRequest, transport?: Transport) =>
  createConnectQueryKey({
    schema: AuthService.method.listMembers,
    input,
    transport,
    cardinality: "finite",
  });

export const createListServiceAccountsQueryKey = (
  input?: ListServiceAccountsRequest,
  transport?: Transport,
) =>
  createConnectQueryKey({
    schema: AuthService.method.listServiceAccounts,
    input,
    transport,
    cardinality: "finite",
  });

export const createGetAuthConfigQueryOptions = ({ transport }: { transport: Transport }) => {
  return {
    ...createQueryOptions(AuthService.method.getAuthConfig, {}, { transport }),
    ...PROBE_QUERY_OPTIONS,
  };
};

export const createGetSessionQueryOptions = ({ transport }: { transport: Transport }) => {
  return {
    ...createQueryOptions(AuthService.method.getSession, {}, { transport }),
    ...PROBE_QUERY_OPTIONS,
  };
};

export const useGetSessionQuery = ({
  options = {},
}: {
  options?: UseQueryOptions<typeof AuthService.method.getSession.output, GetSessionResponse>;
} = {}) => {
  return useQuery(AuthService.method.getSession, {}, { ...PROBE_QUERY_OPTIONS, ...options });
};

const selectCanManageTeam = (response: ListMembersResponse) => response.canManage;

export const useCanManageTeam = () =>
  useQuery(AuthService.method.listMembers, {}, { select: selectCanManageTeam }).data;

export const useListMembersQuery = ({
  options = {},
}: {
  options?: UseQueryOptions<typeof AuthService.method.listMembers.output, ListMembersResponse>;
} = {}) => {
  return useQuery(AuthService.method.listMembers, {}, options);
};

export const useListServiceAccountsQuery = ({
  options = {},
}: {
  options?: UseQueryOptions<
    typeof AuthService.method.listServiceAccounts.output,
    ListServiceAccountsResponse
  >;
} = {}) => {
  return useQuery(AuthService.method.listServiceAccounts, {}, options);
};

export const useLoginMutation = (
  options: UseMutationOptions<
    typeof AuthService.method.login.input,
    typeof AuthService.method.login.output
  > = {},
) => {
  return useMutation(AuthService.method.login, options);
};

export const useLogoutMutation = (
  options: UseMutationOptions<
    typeof AuthService.method.logout.input,
    typeof AuthService.method.logout.output
  > = {},
) => {
  return useMutation(AuthService.method.logout, options);
};

export const useRegisterMutation = (
  options: UseMutationOptions<
    typeof AuthService.method.register.input,
    typeof AuthService.method.register.output
  > = {},
) => {
  return useMutation(AuthService.method.register, options);
};

export const useAcceptInviteMutation = (
  options: UseMutationOptions<
    typeof AuthService.method.acceptInvite.input,
    typeof AuthService.method.acceptInvite.output
  > = {},
) => {
  return useMutation(AuthService.method.acceptInvite, options);
};

export const useInviteMemberMutation = (
  options: UseMutationOptions<
    typeof AuthService.method.inviteMember.input,
    typeof AuthService.method.inviteMember.output
  > = {},
) => {
  const queryClient = useQueryClient();
  const transport = useTransport();
  return useMutation(AuthService.method.inviteMember, {
    ...options,
    onSettled: (...args) => {
      void queryClient.invalidateQueries({
        queryKey: createListMembersQueryKey(undefined, transport),
      });
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
  const transport = useTransport();
  return useMutation(AuthService.method.setMemberRole, {
    ...options,
    onSettled: (...args) => {
      void queryClient.invalidateQueries({
        queryKey: createListMembersQueryKey(undefined, transport),
      });
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
  const transport = useTransport();
  return useMutation(AuthService.method.removeMember, {
    ...options,
    onSettled: (...args) => {
      void queryClient.invalidateQueries({
        queryKey: createListMembersQueryKey(undefined, transport),
      });
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
  const transport = useTransport();
  return useMutation(AuthService.method.createServiceAccount, {
    ...options,
    onSettled: (...args) => {
      void queryClient.invalidateQueries({
        queryKey: createListServiceAccountsQueryKey(undefined, transport),
      });
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
  return useMutation(AuthService.method.rotateServiceAccountSecret, options);
};

export const useRemoveServiceAccountMutation = (
  options: UseMutationOptions<
    typeof AuthService.method.removeServiceAccount.input,
    typeof AuthService.method.removeServiceAccount.output
  > = {},
) => {
  const queryClient = useQueryClient();
  const transport = useTransport();
  return useMutation(AuthService.method.removeServiceAccount, {
    ...options,
    onSettled: (...args) => {
      void queryClient.invalidateQueries({
        queryKey: createListServiceAccountsQueryKey(undefined, transport),
      });
      return options.onSettled?.(...args);
    },
  });
};
