import type { Transport } from "@connectrpc/connect";
import {
  createConnectQueryKey,
  createQueryOptions,
  type UseQueryOptions,
  useQuery,
} from "@connectrpc/connect-query";

import type { ListMembersResponse } from "@/gen/auth/v1/members_pb";
import type { ListServiceAccountsResponse } from "@/gen/auth/v1/service_accounts_pb";
import { AuthService } from "@/gen/auth/v1/service_pb";

import { PROBE_QUERY_OPTIONS } from "@/api/queries/constants";

export const createListMembersQueryKey = () =>
  createConnectQueryKey({ schema: AuthService.method.listMembers, cardinality: "finite" });

export const createListServiceAccountsQueryKey = () =>
  createConnectQueryKey({ schema: AuthService.method.listServiceAccounts, cardinality: "finite" });

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

export const useListMembersQuery = ({
  options = {},
}: {
  options?: UseQueryOptions<typeof AuthService.method.listMembers.output, ListMembersResponse>;
} = {}) => {
  return useQuery<
    typeof AuthService.method.listMembers.input,
    typeof AuthService.method.listMembers.output
  >(AuthService.method.listMembers, {}, options);
};

export const useListServiceAccountsQuery = ({
  options = {},
}: {
  options?: UseQueryOptions<
    typeof AuthService.method.listServiceAccounts.output,
    ListServiceAccountsResponse
  >;
} = {}) => {
  return useQuery<
    typeof AuthService.method.listServiceAccounts.input,
    typeof AuthService.method.listServiceAccounts.output
  >(AuthService.method.listServiceAccounts, {}, options);
};
