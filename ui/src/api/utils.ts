import { create, type DescMessage, type MessageInitShape } from "@bufbuild/protobuf";
import type { UseInfiniteQueryOptions as ConnectUseInfiniteQueryOptions } from "@connectrpc/connect-query";

import type { PaginationRequestSchema, PaginationResponse } from "@/gen/ingestion/v1/pagination_pb";
import { SortBy, SortingRequestSchema, SortOrder } from "@/gen/ingestion/v1/sorting_pb";

import type { ListSearchParams } from "@/module/schemas";

const DEFAULT_PAGE_SIZE = 25;

export const INITIAL_PAGE_PARAM: MessageInitShape<typeof PaginationRequestSchema> = {
  pageSize: DEFAULT_PAGE_SIZE,
};

export const getNextPageParam = (lastPage: { pagination?: PaginationResponse }) => {
  return lastPage.pagination?.nextCursor
    ? {
        pageSize: DEFAULT_PAGE_SIZE,
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

const DEFAULT_LIST_SORT_BY = SortBy.CREATED_AT;
const DEFAULT_LIST_SORT_ORDER = SortOrder.DESC;
export const MAX_LIST_SEARCH_LENGTH = 256;

export const createListSortingInput = ({
  sortBy,
  sortOrder,
}: Pick<ListSearchParams, "sortBy" | "sortOrder">) =>
  create(SortingRequestSchema, {
    sortBy: sortBy ?? DEFAULT_LIST_SORT_BY,
    sortOrder: sortOrder ?? DEFAULT_LIST_SORT_ORDER,
  });

export const createListSearchInput = ({ q, sortBy, sortOrder }: ListSearchParams) => ({
  search: q?.trim().slice(0, MAX_LIST_SEARCH_LENGTH) ?? "",
  sorting: createListSortingInput({ sortBy, sortOrder }),
});

const BATCH_TIMEOUT = Symbol("batchTimeout");

const batchTimeout = (ms: number) =>
  new Promise<typeof BATCH_TIMEOUT>((resolve) => {
    setTimeout(() => resolve(BATCH_TIMEOUT), ms);
  });

export async function* batchIterable<T>(
  source: AsyncIterable<T>,
  intervalMs: number,
): AsyncGenerator<T[]> {
  const iterator = source[Symbol.asyncIterator]();
  let pending: Promise<IteratorResult<T>> | null = null;
  try {
    while (true) {
      const first = await (pending ?? iterator.next());
      pending = null;
      if (first.done) return;
      const batch = [first.value];
      const deadline = Date.now() + intervalMs;
      let remaining = intervalMs;
      while (remaining > 0) {
        pending = iterator.next();
        const result = await Promise.race([pending, batchTimeout(remaining)]);
        if (result === BATCH_TIMEOUT) break;
        pending = null;
        if (result.done) {
          yield batch;
          return;
        }
        batch.push(result.value);
        remaining = deadline - Date.now();
      }
      yield batch;
    }
  } finally {
    pending?.catch(() => {});
    await iterator.return?.();
  }
}
