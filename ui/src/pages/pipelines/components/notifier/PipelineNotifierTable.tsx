import { useState } from "react";

import { PlusIcon } from "@phosphor-icons/react";

import Button, { ButtonSize, ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import Skeleton from "@galaxy-io/dls/feedback/Skeleton";
import SwitchInput, { SwitchInputSize } from "@galaxy-io/dls/inputs/SwitchInput";
import Box, { BoxVariant } from "@galaxy-io/dls/layout/Box";
import Divider from "@galaxy-io/dls/layout/Divider";
import Flex, { AlignItems, FlexDirection } from "@galaxy-io/dls/layout/Flex";
import InfiniteTable from "@galaxy-io/dls/table/InfiniteTable";
import type { TableColumn } from "@galaxy-io/dls/table/types";
import Text, { TextSize } from "@galaxy-io/dls/text/Text";
import Widget, { type WidgetSize } from "@galaxy-io/dls/widget/Widget";

import EmptyLayout from "@/layouts/EmptyLayout";
import { LayoutSize } from "@/layouts/types";

import {
  PIPELINE_NOTIFIER_DEFAULT_STATE,
  PIPELINE_NOTIFIER_TABLE_COLUMN_WIDTH_ENABLED,
} from "@/pages/pipelines/components/notifier/constants";
import PipelineNotifierForm from "@/pages/pipelines/components/notifier/PipelineNotifierForm";
import type {
  PipelineNotifier,
  PipelineNotifierState,
} from "@/pages/pipelines/components/notifier/types";

interface PipelineNotifierTableProps<TRow extends PipelineNotifier> {
  header: string;
  size?: WidgetSize;
  rows: TRow[];
  isLoading?: boolean;
  isSaving?: boolean;
  onCreate: (state: PipelineNotifierState, onSuccess: () => void) => void;
  onUpdate: (row: TRow, state: PipelineNotifierState, onSuccess: () => void) => void;
  onDelete: (row: TRow) => void;
  onToggleEnabled: (row: TRow, isEnabled: boolean) => void;
}

interface PipelineNotifierTableState {
  isOpen: boolean;
  isCreating: boolean;
  expandedRowIds: PipelineNotifier["id"][];
}

const DEFAULT_STATE: PipelineNotifierTableState = {
  isOpen: false,
  isCreating: false,
  expandedRowIds: [],
};

const PipelineNotifierTable = <TRow extends PipelineNotifier>({
  header,
  size,
  rows,
  isLoading = false,
  isSaving = false,
  onCreate,
  onUpdate,
  onDelete,
  onToggleEnabled,
}: PipelineNotifierTableProps<TRow>) => {
  const [state, setState] = useState<PipelineNotifierTableState>(DEFAULT_STATE);

  const handleToggle = () => {
    setState((prev) => ({ ...prev, isOpen: !prev.isOpen }));
  };

  const handleCreatingChange = (isCreating: boolean) => {
    setState((prev) => ({
      ...prev,
      isCreating,
      isOpen: isCreating || prev.isOpen,
      expandedRowIds: isCreating ? [] : prev.expandedRowIds,
    }));
  };

  const handleExpandedChange = (expandedRowIds: PipelineNotifier["id"][]) => {
    setState((prev) => ({ ...prev, expandedRowIds }));
  };

  const columns: TableColumn<TRow>[] = [
    {
      id: "name",
      header: "Name",
      // @dls-migrate infinitetable.column.row-original: `row` is now the data object: `row.original` → `row`.
      cell: ({ row }) => (
        <Text size={TextSize.BODY_SM} lineClamp={1}>
          {row.original.name}
        </Text>
      ),
    },
    {
      id: "enabled",
      header: "",
      align: "right",
      width: PIPELINE_NOTIFIER_TABLE_COLUMN_WIDTH_ENABLED,
      // @dls-migrate infinitetable.column.row-original: `row` is now the data object: `row.original` → `row`.
      cell: ({ row }) => (
        <SwitchInput
          size={SwitchInputSize.LARGE}
          isChecked={row.original.isEnabled}
          onChange={(isEnabled) => onToggleEnabled(row.original, isEnabled)}
          isDisabled={isSaving}
        />
      ),
    },
  ];

  return (
    <Widget
      isCollapsible
      header={header}
      size={size}
      isFlush
      isOpen={state.isOpen}
      onOpenChange={handleToggle}
      actions={
        <Button
          label="Add notifier"
          icon={PlusIcon}
          variant={ButtonVariant.SECONDARY}
          size={ButtonSize.SMALL}
          onClick={() => handleCreatingChange(true)}
          isDisabled={state.isCreating || isSaving}
        />
      }
    >
      <Flex alignItems={AlignItems.START} direction={FlexDirection.COLUMN} fillWidth>
        {state.isCreating && (
          <>
            <PipelineNotifierForm
              initialState={PIPELINE_NOTIFIER_DEFAULT_STATE}
              isSaving={isSaving}
              onSave={(next) => onCreate(next, () => handleCreatingChange(false))}
              onCancel={() => handleCreatingChange(false)}
            />
            <Divider />
          </>
        )}
        <Box variant={BoxVariant.PRIMARY}>
          <InfiniteTable<TRow>
            columns={columns}
            data={rows}
            getRowId={(row) => row.id}
            isLoading={isLoading}
            emptyState={
              <EmptyLayout
                size={LayoutSize.SMALL}
                header="No notifiers"
                message="Add a notifier to get notified when runs complete or fail."
              />
            }
            expandedIds={state.expandedRowIds}
            onExpandedIdsChange={handleExpandedChange}
            renderExpandedRow={(row) => (
              <PipelineNotifierForm
                initialState={row.original}
                isSaving={isSaving}
                onSave={(next) => onUpdate(row.original, next, () => handleExpandedChange([]))}
                onCancel={() => handleExpandedChange([])}
                onDelete={() => onDelete(row.original)}
              />
            )}
            /* @dls-migrate infinitetable.noHeader: Removed: a header-less key–value grid is a `DescriptionList`. */ noHeader
          />
        </Box>
      </Flex>
    </Widget>
  );
};

export default PipelineNotifierTable;
