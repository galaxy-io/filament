import { useMemo } from "react";

import { styled } from "@linaria/react";
import { MagnifyingGlassIcon } from "@phosphor-icons/react";

import FlexWrapper, { AlignItems, FlexDirection } from "@galaxy-io/dls/containers/FlexWrapper";
import Icon, { IconVariant } from "@galaxy-io/dls/icons/Icon";
import InfiniteTable, {
  type ColumnDef,
  ColumnPin,
  type Row,
  TableVariant,
} from "@galaxy-io/dls/table/InfiniteTable";
import Text, { TextSize } from "@galaxy-io/dls/text/Text";
import TextShimmer from "@galaxy-io/dls/text/TextShimmer";

import type { ReadMode } from "@/gen/ingestion/v1/common_pb";

import EmptyLayout from "@/layouts/EmptyLayout";

import CreatePipelineModalResourcesCursorCell from "@/pages/pipelines/components/create/CreatePipelineModalResourcesCursorCell";
import CreatePipelineModalResourcesReadModeCell from "@/pages/pipelines/components/create/CreatePipelineModalResourcesReadModeCell";
import {
  CREATE_PIPELINE_MODAL_COLUMN_WIDTH_CURSOR,
  CREATE_PIPELINE_MODAL_COLUMN_WIDTH_READ_MODE,
  CREATE_PIPELINE_MODAL_RESOURCE_LOADING_ROW_COUNT,
} from "@/pages/pipelines/components/create/constants";
import type { CreatePipelineModalResourceRow } from "@/pages/pipelines/components/create/types";

const TableWrapper = styled.div`
  width: 100%;
  flex: 1;
  min-height: 0;
`;

const NameWrapper = styled.div`
  min-width: 0;
  overflow: hidden;
`;

const NAME_COLUMN: ColumnDef<CreatePipelineModalResourceRow> = {
  id: "name",
  header: "Resource",
  accessorFn: (row) => row.displayName,
  enableSorting: true,
  cellLoading: () => <TextShimmer width={180} height={14} />,
  cell: ({ row }) => (
    <NameWrapper>
      <Text size={TextSize.BODY_SM} isMonospace isEllipsis>
        {row.original.displayName}
      </Text>
    </NameWrapper>
  ),
};

interface CreatePipelineModalResourcesTableProps {
  rows: CreatePipelineModalResourceRow[];
  hasLevers: boolean;
  isLoading: boolean;
  onSelectionChange: (visibleNames: string[], selection: Record<string, boolean>) => void;
  onReadModeChange: (resource: string, readMode: ReadMode) => void;
  onCursorChange: (resource: string, cursorField: string) => void;
}

const CreatePipelineModalResourcesTable = ({
  rows,
  hasLevers,
  isLoading,
  onSelectionChange,
  onReadModeChange,
  onCursorChange,
}: CreatePipelineModalResourcesTableProps) => {
  const rowSelection = useMemo(
    () => Object.fromEntries(rows.map((row) => [row.name, row.isSelected])),
    [rows],
  );

  const columns = useMemo<ColumnDef<CreatePipelineModalResourceRow>[]>(() => {
    if (!hasLevers) return [NAME_COLUMN];

    return [
      NAME_COLUMN,
      {
        id: "readMode",
        header: "Read mode",
        size: CREATE_PIPELINE_MODAL_COLUMN_WIDTH_READ_MODE,
        pin: ColumnPin.RIGHT,
        cellLoading: () => <TextShimmer width={120} height={24} />,
        cell: ({ row }) => (
          <CreatePipelineModalResourcesReadModeCell
            row={row.original}
            onChange={onReadModeChange}
          />
        ),
      },
      {
        id: "cursor",
        header: "Cursor",
        size: CREATE_PIPELINE_MODAL_COLUMN_WIDTH_CURSOR,
        pin: ColumnPin.RIGHT,
        cellLoading: () => <TextShimmer width={140} height={24} />,
        cell: ({ row }) => (
          <CreatePipelineModalResourcesCursorCell row={row.original} onChange={onCursorChange} />
        ),
      },
    ];
  }, [hasLevers, onReadModeChange, onCursorChange]);

  return (
    <TableWrapper>
      <InfiniteTable<CreatePipelineModalResourceRow>
        variant={TableVariant.PRIMARY}
        columns={columns}
        data={rows}
        getRowId={(row) => row.name}
        isLoading={isLoading}
        loadingRowCount={CREATE_PIPELINE_MODAL_RESOURCE_LOADING_ROW_COUNT}
        rowSelection={rowSelection}
        onRowSelectionChange={(updater) =>
          onSelectionChange(
            rows.map((row) => row.name),
            typeof updater === "function" ? updater(rowSelection) : updater,
          )
        }
        enableRowSelection={(row: Row<CreatePipelineModalResourceRow>) => row.original.isSelectable}
        getRowSelectAriaLabel={(row: Row<CreatePipelineModalResourceRow>) => row.original.name}
        contentWhenEmpty={
          <FlexWrapper
            direction={FlexDirection.COLUMN}
            alignItems={AlignItems.CENTER}
            padding={24}
            fillWidth
          >
            <EmptyLayout
              icon={<Icon component={MagnifyingGlassIcon} variant={IconVariant.TERTIARY} />}
              message="No resources match your search"
            />
          </FlexWrapper>
        }
        showSelectionColumn
        enableSelectAll
        enableSelectionRange
        fillWidth
        fillHeight
      />
    </TableWrapper>
  );
};

export default CreatePipelineModalResourcesTable;
