import type { InfiniteTableProps } from "@galaxy-io/dls/table/InfiniteTable";

import type { Pipeline } from "@/gen/ingestion/v1/pipelines_pb";
import { SortBy, SortOrder } from "@/gen/ingestion/v1/sorting_pb";

import { DEFAULT_LIST_SORT_BY, DEFAULT_LIST_SORT_ORDER, type ListSearchParams } from "@/api/utils";

export type PipelinesTableSorting = NonNullable<InfiniteTableProps<Pipeline>["sorting"]>;

export type PipelinesTableSortingChange = NonNullable<
  InfiniteTableProps<Pipeline>["onSortingChange"]
>;

export const PIPELINES_TABLE_COLUMN_ID_PIPELINE = "pipeline";

const PIPELINES_TABLE_COLUMN_ID_TO_SORT_BY_MAP: Record<string, SortBy | undefined> = {
  [PIPELINES_TABLE_COLUMN_ID_PIPELINE]: SortBy.NAME,
};

const PIPELINES_TABLE_SORT_BY_TO_COLUMN_ID_MAP: Partial<Record<SortBy, string>> = {
  [SortBy.NAME]: PIPELINES_TABLE_COLUMN_ID_PIPELINE,
};

export const createPipelinesTableSorting = ({
  sortBy,
  sortOrder,
}: ListSearchParams): PipelinesTableSorting => {
  const id = PIPELINES_TABLE_SORT_BY_TO_COLUMN_ID_MAP[sortBy ?? DEFAULT_LIST_SORT_BY];
  if (!id) {
    return [];
  }
  return [{ id, desc: (sortOrder ?? DEFAULT_LIST_SORT_ORDER) === SortOrder.DESC }];
};

export const createPipelinesTableSortSearch = (
  sorting: PipelinesTableSorting,
): Pick<ListSearchParams, "sortBy" | "sortOrder"> => {
  const [column] = sorting;
  const sortBy = column ? PIPELINES_TABLE_COLUMN_ID_TO_SORT_BY_MAP[column.id] : undefined;
  if (!sortBy) {
    return { sortBy: undefined, sortOrder: undefined };
  }
  const sortOrder = column.desc ? SortOrder.DESC : SortOrder.ASC;
  if (sortBy === DEFAULT_LIST_SORT_BY && sortOrder === DEFAULT_LIST_SORT_ORDER) {
    return { sortBy: undefined, sortOrder: undefined };
  }
  return { sortBy, sortOrder };
};
