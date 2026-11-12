import { type FC, useMemo } from "react";

import { MagnifyingGlassIcon } from "@phosphor-icons/react";

import Chip, { ChipSize, ChipVariant } from "@galaxy-io/dls/chips/Chip";
import EmptyLayout, { EmptyLayoutSize } from "@galaxy-io/dls/layout/EmptyLayout";
import Flex, { AlignItems, JustifyContent } from "@galaxy-io/dls/layout/Flex";
import InfiniteTable, { InfiniteTableSize } from "@galaxy-io/dls/table/InfiniteTable";
import type { TableColumn } from "@galaxy-io/dls/table/types";
import Text, { TextSize, TextVariant } from "@galaxy-io/dls/text/Text";
import { FontFamily } from "@galaxy-io/dls/theme/enums";
import { isSearchMatch } from "@galaxy-io/dls/utils/search";

import type { ResourceColumn } from "@/gen/ingestion/v1/connectors_pb";

import {
  PIPELINE_RESOURCE_COLUMNS_TABLE_INSET,
  PIPELINE_RESOURCE_COLUMNS_TABLE_WIDTH_TYPE,
} from "@/pages/pipelines/components/resource/constants";

const RESOURCE_COLUMN_TABLE_COLUMNS: TableColumn<ResourceColumn>[] = [
  {
    id: "name",
    header: "Column",
    isRowHeader: true,
    cell: ({ row }) => (
      <Flex alignItems={AlignItems.CENTER} gap={8} minWidth={0}>
        <Text size={TextSize.BODY_SM} family={FontFamily.MONO} lineClamp={1}>
          {row.name}
        </Text>
        {row.isPrimaryKey && (
          <Chip
            label="primary key"
            tooltip="Part of the primary key"
            size={ChipSize.SMALL}
            variant={ChipVariant.TERTIARY}
          />
        )}
      </Flex>
    ),
  },
  {
    id: "type",
    header: "Type",
    width: PIPELINE_RESOURCE_COLUMNS_TABLE_WIDTH_TYPE,
    align: "right",
    cell: ({ row }) => (
      <Flex justifyContent={JustifyContent.END} fillWidth>
        <Text
          size={TextSize.BODY_SM}
          family={FontFamily.MONO}
          variant={TextVariant.TERTIARY}
          lineClamp={1}
        >
          {row.nativeType || row.logicalType}
        </Text>
      </Flex>
    ),
  },
];

interface PipelineResourceColumnsTableProps {
  columns: ResourceColumn[];
  selected: ResourceColumn["name"][];
  locked: ResourceColumn["name"][];
  isDisabled: boolean;
  search?: string;
  onChange: (selected: ResourceColumn["name"][]) => void;
}

const PipelineResourceColumnsTable: FC<PipelineResourceColumnsTableProps> = ({
  columns,
  selected,
  locked,
  isDisabled,
  search = "",
  onChange,
}) => {
  const visibleColumns = useMemo(
    () => columns.filter((column) => isSearchMatch(search, column.name)),
    [columns, search],
  );

  const handleChange = (next: ResourceColumn["name"][]) => {
    const visibleNames = new Set(visibleColumns.map((column) => column.name));
    const kept = columns
      .map((column) => column.name)
      .filter(
        (name) =>
          next.includes(name) ||
          locked.includes(name) ||
          (!visibleNames.has(name) && selected.includes(name)),
      );
    if (kept.length > 0) onChange(kept);
  };

  return (
    <InfiniteTable<ResourceColumn>
      ariaLabel="Columns"
      columns={RESOURCE_COLUMN_TABLE_COLUMNS}
      data={visibleColumns}
      getRowId={(row) => row.name}
      size={InfiniteTableSize.SMALL}
      inset={PIPELINE_RESOURCE_COLUMNS_TABLE_INSET}
      value={selected}
      onChange={isDisabled ? undefined : handleChange}
      canSelectRow={(column) => !locked.includes(column.name)}
      emptyState={
        <EmptyLayout
          size={EmptyLayoutSize.SMALL}
          icon={MagnifyingGlassIcon}
          header="No columns match your search"
        />
      }
      isSelectable
    />
  );
};

export default PipelineResourceColumnsTable;
