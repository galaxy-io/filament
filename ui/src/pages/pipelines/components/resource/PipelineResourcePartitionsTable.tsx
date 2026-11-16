import { type FC, useMemo } from "react";

import { MagnifyingGlassIcon } from "@phosphor-icons/react";

import EmptyLayout, { EmptyLayoutSize } from "@galaxy-io/dls/layout/EmptyLayout";
import InfiniteTable, { InfiniteTableSize } from "@galaxy-io/dls/table/InfiniteTable";
import type { TableColumn } from "@galaxy-io/dls/table/types";
import Text, { TextSize } from "@galaxy-io/dls/text/Text";
import { FontFamily } from "@galaxy-io/dls/theme/enums";
import { isSearchMatch } from "@galaxy-io/dls/utils/search";

import type { ResourceColumns } from "@/gen/ingestion/v1/connectors_pb";

import { PIPELINE_RESOURCE_COLUMNS_TABLE_INSET } from "@/pages/pipelines/components/resource/constants";
import type { PipelineResourcePartitionRow } from "@/pages/pipelines/components/resource/types";

const RESOURCE_PARTITION_TABLE_COLUMNS: TableColumn<PipelineResourcePartitionRow>[] = [
  {
    id: "name",
    header: "Partition",
    isRowHeader: true,
    cell: ({ row }) => (
      <Text size={TextSize.BODY_SM} family={FontFamily.MONO} lineClamp={1}>
        {row.name}
      </Text>
    ),
  },
];

interface PipelineResourcePartitionsTableProps {
  partitions: ResourceColumns["partitions"];
  selected: ResourceColumns["partitions"];
  isDisabled: boolean;
  search?: string;
  onChange: (selected: ResourceColumns["partitions"]) => void;
}

const PipelineResourcePartitionsTable: FC<PipelineResourcePartitionsTableProps> = ({
  partitions,
  selected,
  isDisabled,
  search = "",
  onChange,
}) => {
  const rows = useMemo(
    () => partitions.filter((name) => isSearchMatch(search, name)).map((name) => ({ name })),
    [partitions, search],
  );

  const handleChange = (next: ResourceColumns["partitions"]) => {
    const visible = new Set(rows.map((row) => row.name));
    const kept = partitions.filter(
      (name) => next.includes(name) || (!visible.has(name) && selected.includes(name)),
    );
    onChange(kept);
  };

  return (
    <InfiniteTable<PipelineResourcePartitionRow>
      ariaLabel="Partitions"
      columns={RESOURCE_PARTITION_TABLE_COLUMNS}
      data={rows}
      getRowId={(row) => row.name}
      size={InfiniteTableSize.SMALL}
      inset={PIPELINE_RESOURCE_COLUMNS_TABLE_INSET}
      value={selected}
      onChange={isDisabled ? undefined : handleChange}
      emptyState={
        <EmptyLayout
          size={EmptyLayoutSize.SMALL}
          icon={MagnifyingGlassIcon}
          header="No partitions match your search"
        />
      }
      isSelectable
    />
  );
};

export default PipelineResourcePartitionsTable;
