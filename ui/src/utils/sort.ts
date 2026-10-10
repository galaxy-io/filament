import type { TableSort } from "@galaxy-io/dls/table/types";

import { SortBy, SortOrder } from "@/gen/ingestion/v1/sorting_pb";

import type { ListSearchParams } from "@/module/schemas";

import { getEnumValues } from "@/utils/select";

type ListSortSearch = Pick<ListSearchParams, "sortBy" | "sortOrder">;

export const createTableSorting = (
  { sortBy, sortOrder }: ListSortSearch,
  sortByToColumnId: Record<SortBy, string | undefined>,
): TableSort | null => {
  const columnId = sortBy === undefined ? undefined : sortByToColumnId[sortBy];
  return columnId ? { columnId, isDescending: sortOrder !== SortOrder.ASC } : null;
};

export const createTableSortSearch = (
  sorting: TableSort | null,
  sortByToColumnId: Record<SortBy, string | undefined>,
): ListSortSearch => {
  const sortBy = getEnumValues(SortBy).find(
    (value) => sortByToColumnId[value] === sorting?.columnId,
  );
  if (!sorting || sortBy === undefined) {
    return { sortBy: undefined, sortOrder: undefined };
  }
  return { sortBy, sortOrder: sorting.isDescending ? SortOrder.DESC : SortOrder.ASC };
};
