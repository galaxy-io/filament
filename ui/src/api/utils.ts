import type { DescMessage, MessageInitShape } from "@bufbuild/protobuf";
import type { UseInfiniteQueryOptions as ConnectUseInfiniteQueryOptions } from "@connectrpc/connect-query";

import type { PaginationRequestSchema, PaginationResponse } from "@/gen/ingestion/v1/pagination_pb";

export const DEFAULT_PAGINATION_TOTAL = 25;

export const INITIAL_PAGE_PARAM: MessageInitShape<typeof PaginationRequestSchema> = {
  total: DEFAULT_PAGINATION_TOTAL,
};

export const getNextPageParam = (lastPage: { pagination?: PaginationResponse }) => {
  return lastPage.pagination?.nextCursor
    ? {
        total: DEFAULT_PAGINATION_TOTAL,
        cursor: lastPage.pagination.nextCursor,
      }
    : undefined;
};

export type UseInfiniteQueryOptions<
  I extends DescMessage,
  O extends DescMessage,
  ParamKey extends keyof MessageInitShape<I>,
> = Omit<ConnectUseInfiniteQueryOptions<I, O, ParamKey>, "pageParamKey" | "getNextPageParam">;

export type InfiniteQueryInput<I extends DescMessage> = Omit<
  MessageInitShape<I>,
  "pagination" | "$typeName"
>;
