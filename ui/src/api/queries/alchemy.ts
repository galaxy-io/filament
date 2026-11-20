import { create } from "@bufbuild/protobuf";
import {
  createConnectQueryKey,
  type UseQueryOptions,
  useInfiniteQuery,
  useQuery,
} from "@connectrpc/connect-query";

import {
  type AlchemyFunction,
  AlchemyVersionStatus,
  GetAlchemyFunctionRequestSchema,
  type GetAlchemyFunctionResponse,
} from "@/gen/ingestion/v1/alchemy_pb";
import { IngestionService } from "@/gen/ingestion/v1/service_pb";

import { ALCHEMY_BUILD_REFETCH_INTERVAL, ALCHEMY_QUERY_OPTIONS } from "@/api/queries/constants";
import {
  getNextPageParam,
  INITIAL_PAGE_PARAM,
  type InfiniteQueryInput,
  MAX_LIST_SEARCH_LENGTH,
} from "@/api/utils";

export const createListAlchemyFunctionsInput = (
  search = "",
): InfiniteQueryInput<typeof IngestionService.method.listAlchemyFunctions.input> => ({
  search: search.trim().slice(0, MAX_LIST_SEARCH_LENGTH),
});

export const createListAlchemyFunctionsQueryKey = () =>
  createConnectQueryKey({
    schema: IngestionService.method.listAlchemyFunctions,
    cardinality: "infinite",
  });

export const useListAlchemyFunctionsInfiniteQuery = ({
  input = createListAlchemyFunctionsInput(),
}: {
  input?: InfiniteQueryInput<typeof IngestionService.method.listAlchemyFunctions.input>;
} = {}) => {
  return useInfiniteQuery(
    IngestionService.method.listAlchemyFunctions,
    { ...input, pagination: INITIAL_PAGE_PARAM },
    { pageParamKey: "pagination", getNextPageParam, ...ALCHEMY_QUERY_OPTIONS },
  );
};

export const createGetAlchemyFunctionInput = (id: AlchemyFunction["id"], version = 0) =>
  create(GetAlchemyFunctionRequestSchema, { id, version: BigInt(version) });

export const useGetAlchemyFunctionQuery = ({
  input,
  options = {},
}: {
  input: ReturnType<typeof createGetAlchemyFunctionInput>;
  options?: UseQueryOptions<
    typeof IngestionService.method.getAlchemyFunction.output,
    GetAlchemyFunctionResponse
  >;
}) => {
  return useQuery(IngestionService.method.getAlchemyFunction, input, {
    ...ALCHEMY_QUERY_OPTIONS,
    refetchInterval: (query) =>
      query.state.data?.versionStatus === AlchemyVersionStatus.BUILDING
        ? ALCHEMY_BUILD_REFETCH_INTERVAL
        : false,
    ...options,
  });
};
