import { useMemo } from "react";

import { styled } from "@linaria/react";
import { MagnifyingGlassIcon } from "@phosphor-icons/react";

import Skeleton from "@galaxy-io/dls/feedback/Skeleton";
import Icon, { IconVariant } from "@galaxy-io/dls/icons/Icon";
import Box, { BoxVariant } from "@galaxy-io/dls/layout/Box";
import Flex, { AlignItems, FlexDirection } from "@galaxy-io/dls/layout/Flex";
// @dls-migrate infinitetable.Row: Removed: TanStack types are not exposed; use `TableColumn`, `TableColumnLayout`, `TableCellContext`, `TableSort`.
import InfiniteTable, { type Row } from "@galaxy-io/dls/table/InfiniteTable";
import type { TableColumn } from "@galaxy-io/dls/table/types";
import Text, { TextSize } from "@galaxy-io/dls/text/Text";
import { FontFamily, Side } from "@galaxy-io/dls/theme/enums";

import EmptyLayout from "@/layouts/EmptyLayout";

import { CreatePipelineModalActionType } from "@/pages/pipelines/components/create/actions";
import {
  useCreatePipelineModalDispatch,
  useCreatePipelineModalState,
} from "@/pages/pipelines/components/create/CreatePipelineModalProvider";
import {
  CREATE_PIPELINE_MODAL_COLUMN_WIDTH_CURSOR,
  CREATE_PIPELINE_MODAL_COLUMN_WIDTH_READ_MODE,
  CREATE_PIPELINE_MODAL_RESOURCE_LOADING_ROW_COUNT,
} from "@/pages/pipelines/components/create/constants";
import CreatePipelineModalResourcesCursorCell from "@/pages/pipelines/components/create/steps/components/CreatePipelineModalResourcesCursorCell";
import CreatePipelineModalResourcesReadModeCell from "@/pages/pipelines/components/create/steps/components/CreatePipelineModalResourcesReadModeCell";
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

const NAME_COLUMN: TableColumn<CreatePipelineModalResourceRow> = {
  id: "name",
  header: "Resource",
  accessor: (row) => row.displayName,
  canSort: true,
  // @dls-migrate infinitetable.column.row-original: `row` is now the data object: `row.original` → `row`.
  cell: ({ row }) => (
    <NameWrapper>
      <Text size={TextSize.BODY_SM} family={FontFamily.MONO} lineClamp={1}>
        {row.original.displayName}
      </Text>
    </NameWrapper>
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
    // @dls-migrate infinitetable.column.row-original: `row` is now the data object: `row.original` → `row`.
    cell: ({ row }) => <CreatePipelineModalResourcesReadModeCell row={row.original} />,
  },
  {
    id: "cursor",
    header: "Cursor",
    width: CREATE_PIPELINE_MODAL_COLUMN_WIDTH_CURSOR,
    pin: Side.RIGHT,
    // @dls-migrate infinitetable.column.row-original: `row` is now the data object: `row.original` → `row`.
    cell: ({ row }) => <CreatePipelineModalResourcesCursorCell row={row.original} />,
  },
];

interface CreatePipelineModalResourcesTableProps {
  rows: CreatePipelineModalResourceRow[];
}

const CreatePipelineModalResourcesTable = ({ rows }: CreatePipelineModalResourcesTableProps) => {
  const { activeSinkId, hasReadLevers, isLoading } = useCreatePipelineModalState();
  const dispatch = useCreatePipelineModalDispatch();

  const rowSelection = useMemo(
    () => Object.fromEntries(rows.map((row) => [row.name, row.isSelected])),
    [rows],
  );

  const columns = hasReadLevers ? RESOURCE_COLUMNS_WITH_LEVERS : RESOURCE_COLUMNS_BASE;

  return (
    <TableWrapper>
      <Box variant={BoxVariant.PRIMARY} height="100%">
        <InfiniteTable<CreatePipelineModalResourceRow>
          columns={columns}
          data={rows}
          getRowId={(row) => row.name}
          isLoading={isLoading}
          /* @dls-migrate infinitetable.rowSelection: Selection is now a `string[]` of row ids. */ rowSelection={
            rowSelection
          }
          /* @dls-migrate infinitetable.onRowSelectionChange: Selection is now a `string[]` of row ids. */ onRowSelectionChange={(
            updater,
          ) =>
            dispatch({
              type: CreatePipelineModalActionType.SET_RESOURCE_SELECTION,
              payload: {
                sinkId: activeSinkId,
                visibleNames: rows.map((row) => row.name),
                selection: typeof updater === "function" ? updater(rowSelection) : updater,
              },
            })
          }
          /* @dls-migrate infinitetable.enableRowSelection-predicate: The per-row predicate is removed: every row can be selected. */ enableRowSelection={(
            row: Row<CreatePipelineModalResourceRow>,
          ) => row.original.isSelectable}
          /* @dls-migrate infinitetable.getRowSelectAriaLabel: Mark the naming column `isRowHeader` instead. */ getRowSelectAriaLabel={(
            row: Row<CreatePipelineModalResourceRow>,
          ) => row.original.name}
          emptyState={
            <Flex
              direction={FlexDirection.COLUMN}
              alignItems={AlignItems.CENTER}
              padding={24}
              fillWidth
            >
              <EmptyLayout
                icon={<Icon component={MagnifyingGlassIcon} variant={IconVariant.TERTIARY} />}
                message="No resources match your search"
              />
            </Flex>
          }
          isSelectable
        />
      </Box>
    </TableWrapper>
  );
};

export default CreatePipelineModalResourcesTable;
