import { type FC, useMemo } from "react";

import { MagnifyingGlassIcon } from "@phosphor-icons/react";

import Box, { BoxVariant } from "@galaxy-io/dls/layout/Box";
import EmptyLayout from "@galaxy-io/dls/layout/EmptyLayout";
import Flex, { AlignItems, FlexDirection } from "@galaxy-io/dls/layout/Flex";
import FlexItem from "@galaxy-io/dls/layout/FlexItem";
import InfiniteTable from "@galaxy-io/dls/table/InfiniteTable";
import type { TableColumn } from "@galaxy-io/dls/table/types";
import Text, { TextSize } from "@galaxy-io/dls/text/Text";
import { FontFamily, Side } from "@galaxy-io/dls/theme/enums";

import {
  useCreatePipelineModalActions,
  useCreatePipelineModalState,
} from "@/pages/pipelines/components/create/CreatePipelineModalProvider";
import {
  CREATE_PIPELINE_MODAL_COLUMN_WIDTH_CURSOR,
  CREATE_PIPELINE_MODAL_COLUMN_WIDTH_READ_MODE,
  CREATE_PIPELINE_MODAL_COLUMN_WIDTH_WRITE_MODE,
} from "@/pages/pipelines/components/create/constants";
import CreatePipelineModalResourcesCursorCell from "@/pages/pipelines/components/create/steps/components/CreatePipelineModalResourcesCursorCell";
import CreatePipelineModalResourcesReadModeCell from "@/pages/pipelines/components/create/steps/components/CreatePipelineModalResourcesReadModeCell";
import CreatePipelineModalResourcesWriteModeCell from "@/pages/pipelines/components/create/steps/components/CreatePipelineModalResourcesWriteModeCell";
import type { CreatePipelineModalResourceRow } from "@/pages/pipelines/components/create/types";
import PipelineResourceColumnsChip from "@/pages/pipelines/components/resource/PipelineResourceColumnsChip";

const NAME_COLUMN: TableColumn<CreatePipelineModalResourceRow> = {
  id: "name",
  header: "Resource",
  accessor: (row) => row.displayName,
  canSort: true,
  isRowHeader: true,
  cell: ({ row }) => (
    <Flex alignItems={AlignItems.CENTER} gap={8} minWidth={0}>
      <FlexItem minWidth={0} overflow="hidden">
        <Text size={TextSize.BODY_SM} family={FontFamily.MONO} lineClamp={1}>
          {row.displayName}
        </Text>
      </FlexItem>
      {row.isColumnSelectable && row.selectedColumns.length !== row.columns.length && (
        <FlexItem shrink={0}>
          <PipelineResourceColumnsChip
            selectedCount={row.selectedColumns.length}
            totalCount={row.columns.length}
          />
        </FlexItem>
      )}
    </Flex>
  ),
};

const RESOURCE_COLUMNS_BASE: TableColumn<CreatePipelineModalResourceRow>[] = [NAME_COLUMN];

const RESOURCE_COLUMNS_WITH_LEVERS: TableColumn<CreatePipelineModalResourceRow>[] = [
  NAME_COLUMN,
  {
    id: "readMode",
    header: "Read mode",
    width: CREATE_PIPELINE_MODAL_COLUMN_WIDTH_READ_MODE,
    pin: Side.RIGHT,
    cell: ({ row }) => <CreatePipelineModalResourcesReadModeCell row={row} />,
  },
  {
    id: "cursor",
    header: "Cursor",
    width: CREATE_PIPELINE_MODAL_COLUMN_WIDTH_CURSOR,
    pin: Side.RIGHT,
    cell: ({ row }) => <CreatePipelineModalResourcesCursorCell row={row} />,
  },
  {
    id: "writeMode",
    header: "Write mode",
    width: CREATE_PIPELINE_MODAL_COLUMN_WIDTH_WRITE_MODE,
    pin: Side.RIGHT,
    cell: ({ row }) => <CreatePipelineModalResourcesWriteModeCell row={row} />,
  },
];

interface CreatePipelineModalResourcesTableProps {
  rows: CreatePipelineModalResourceRow[];
  activeRow?: CreatePipelineModalResourceRow;
  onActiveResourceChange: (name?: CreatePipelineModalResourceRow["name"]) => void;
}

const CreatePipelineModalResourcesTable: FC<CreatePipelineModalResourcesTableProps> = ({
  rows,
  activeRow,
  onActiveResourceChange,
}) => {
  const { activeSinkId, hasReadLevers, isLoading } = useCreatePipelineModalState();
  const { setResourceSelection } = useCreatePipelineModalActions();

  const selectedNames = useMemo(
    () => rows.filter((row) => row.isSelected).map((row) => row.name),
    [rows],
  );

  const handleSelectionChange = (names: string[]) =>
    setResourceSelection({
      sinkId: activeSinkId,
      visibleNames: rows.map((row) => row.name),
      selection: Object.fromEntries(names.map((name) => [name, true])),
    });

  const handleRowClick = (row: CreatePipelineModalResourceRow) => {
    if (!row.isColumnSelectable) return;
    onActiveResourceChange(row.name === activeRow?.name ? undefined : row.name);
  };

  const columns = hasReadLevers ? RESOURCE_COLUMNS_WITH_LEVERS : RESOURCE_COLUMNS_BASE;
  const hasColumnSelection = rows.some((row) => row.isColumnSelectable);

  return (
    <FlexItem grow={1} basis={0} minHeight={0} fillWidth>
      <Box variant={BoxVariant.PRIMARY} height="100%">
        <InfiniteTable<CreatePipelineModalResourceRow>
          ariaLabel="Resources"
          columns={columns}
          data={rows}
          getRowId={(row) => row.name}
          isLoading={isLoading}
          value={selectedNames}
          onChange={handleSelectionChange}
          onRowClick={hasColumnSelection ? handleRowClick : undefined}
          activeRowId={activeRow?.name ?? null}
          emptyState={
            <Flex
              direction={FlexDirection.COLUMN}
              alignItems={AlignItems.CENTER}
              padding={24}
              fillWidth
            >
              <EmptyLayout icon={MagnifyingGlassIcon} header="No resources match your search" />
            </Flex>
          }
          isSelectable
        />
      </Box>
    </FlexItem>
  );
};

export default CreatePipelineModalResourcesTable;
