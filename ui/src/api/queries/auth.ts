import { createConnectQueryKey, type UseQueryOptions, useQuery } from "@connectrpc/connect-query";

import type { ListMembersResponse } from "@/gen/auth/v1/members_pb";
import { AuthService } from "@/gen/auth/v1/service_pb";
import type { GetAuthConfigResponse } from "@/gen/auth/v1/session_pb";

export const createGetAuthConfigQueryKey = () =>
  createConnectQueryKey({ schema: AuthService.method.getAuthConfig, cardinality: "finite" });

export const createListMembersQueryKey = () =>
  createConnectQueryKey({ schema: AuthService.method.listMembers, cardinality: "finite" });

export const useGetAuthConfigQuery = ({
  options = {},
}: {
  options?: UseQueryOptions<typeof AuthService.method.getAuthConfig.output, GetAuthConfigResponse>;
} = {}) => {
  return useQuery<
    typeof AuthService.method.getAuthConfig.input,
    typeof AuthService.method.getAuthConfig.output
  >(AuthService.method.getAuthConfig, {}, options);
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
