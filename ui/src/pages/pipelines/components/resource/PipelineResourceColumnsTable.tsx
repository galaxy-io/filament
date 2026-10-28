import { type FC, useMemo } from "react";

import Chip, { ChipSize, ChipVariant } from "@galaxy-io/dls/chips/Chip";
import Flex, { AlignItems, JustifyContent } from "@galaxy-io/dls/layout/Flex";
import InfiniteTable, { InfiniteTableSize } from "@galaxy-io/dls/table/InfiniteTable";
import type { TableColumn } from "@galaxy-io/dls/table/types";
import Text, { TextSize, TextVariant } from "@galaxy-io/dls/text/Text";
import { FontFamily } from "@galaxy-io/dls/theme/enums";

import type { ResourceColumn } from "@/gen/ingestion/v1/connectors_pb";

import {
  PIPELINE_RESOURCE_COLUMNS_TABLE_INSET,
  PIPELINE_RESOURCE_COLUMNS_TABLE_WIDTH_TYPE,
} from "@/pages/pipelines/components/resource/constants";

interface PipelineResourceColumnsTableProps {
  columns: ResourceColumn[];
  selected: ResourceColumn["name"][];
  locked: ResourceColumn["name"][];
  isDisabled: boolean;
  onChange: (selected: ResourceColumn["name"][]) => void;
}

const PipelineResourceColumnsTable: FC<PipelineResourceColumnsTableProps> = ({
  columns,
  selected,
  locked,
  isDisabled,
  onChange,
}) => {
  const tableColumns = useMemo<TableColumn<ResourceColumn>[]>(
    () => [
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
              <Chip label="key" size={ChipSize.SMALL} variant={ChipVariant.SECONDARY} />
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
    ],
    [],
  );

  const handleChange = (next: ResourceColumn["name"][]) =>
    onChange(
      columns
        .map((column) => column.name)
        .filter((name) => next.includes(name) || locked.includes(name)),
    );

  return (
    <InfiniteTable<ResourceColumn>
      ariaLabel="Columns"
      columns={tableColumns}
      data={columns}
      getRowId={(row) => row.name}
      size={InfiniteTableSize.SMALL}
      inset={PIPELINE_RESOURCE_COLUMNS_TABLE_INSET}
      value={selected}
      onChange={isDisabled ? undefined : handleChange}
      isSelectable
    />
  );
};

export default PipelineResourceColumnsTable;
