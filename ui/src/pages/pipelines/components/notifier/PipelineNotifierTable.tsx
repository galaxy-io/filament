import { useState } from "react";

import { PlusIcon } from "@phosphor-icons/react";

import Button, { ButtonSize, ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import Chip, { ChipSize, ChipVariant } from "@galaxy-io/dls/chips/Chip";
import SwitchInput from "@galaxy-io/dls/inputs/SwitchInput";
import Box, { BoxVariant } from "@galaxy-io/dls/layout/Box";
import Divider from "@galaxy-io/dls/layout/Divider";
import EmptyLayout, { EmptyLayoutSize } from "@galaxy-io/dls/layout/EmptyLayout";
import Flex, { AlignItems, FlexDirection } from "@galaxy-io/dls/layout/Flex";
import InfiniteTable from "@galaxy-io/dls/table/InfiniteTable";
import type { TableColumn } from "@galaxy-io/dls/table/types";
import Text, { TextSize } from "@galaxy-io/dls/text/Text";
import Widget from "@galaxy-io/dls/widget/Widget";

import {
  PIPELINE_NOTIFIER_DEFAULT_STATE,
  PIPELINE_NOTIFIER_TABLE_COLUMN_WIDTH_ENABLED,
  PIPELINE_NOTIFIER_TABLE_EMPTY_HEIGHT,
} from "@/pages/pipelines/components/notifier/constants";
import PipelineNotifierForm from "@/pages/pipelines/components/notifier/PipelineNotifierForm";
import PipelineNotifierTypeTile from "@/pages/pipelines/components/notifier/PipelineNotifierTypeTile";
import type {
  PipelineNotifier,
  PipelineNotifierState,
} from "@/pages/pipelines/components/notifier/types";

interface PipelineNotifierTableProps<TRow extends PipelineNotifier> {
  header?: string;
  rows: TRow[];
  isOpenInitial?: boolean;
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

const PipelineNotifierTable = <TRow extends PipelineNotifier>({
  header = "Notifiers",
  rows,
  isOpenInitial = true,
  isLoading = false,
  isSaving = false,
  onCreate,
  onUpdate,
  onDelete,
  onToggleEnabled,
}: PipelineNotifierTableProps<TRow>) => {
  const [state, setState] = useState<PipelineNotifierTableState>(() => ({
    isOpen: isOpenInitial,
    isCreating: false,
    expandedRowIds: [],
  }));

  const handleOpenChange = (isOpen: boolean) => {
    setState((prev) => ({ ...prev, isOpen }));
  };

  const handleCreatingChange = (isCreating: boolean) => {
    setState((prev) => ({
      ...prev,
      isOpen: prev.isOpen || isCreating,
      isCreating,
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
      cell: ({ row }) => (
        <Flex alignItems={AlignItems.CENTER} gap={8}>
          <PipelineNotifierTypeTile notificationType={row.notificationType} />
          <Text size={TextSize.BODY_SM} lineClamp={1}>
            {row.name}
          </Text>
        </Flex>
      ),
    },
    {
      id: "enabled",
      header: "",
      align: "right",
      width: PIPELINE_NOTIFIER_TABLE_COLUMN_WIDTH_ENABLED,
      cell: ({ row }) => (
        <SwitchInput
          isChecked={row.isEnabled}
          onChange={(isEnabled) => onToggleEnabled(row, isEnabled)}
          isDisabled={isSaving}
        />
      ),
    },
  ];

  const hasRows = isLoading || rows.length > 0;

  return (
    <Widget
      isCollapsible
      header={header}
      isFlush
      isOpen={state.isOpen}
      onOpenChange={handleOpenChange}
      actions={
        <Flex alignItems={AlignItems.CENTER} gap={8}>
          <Button
            label="Add notifier"
            icon={PlusIcon}
            variant={ButtonVariant.SECONDARY}
            size={ButtonSize.SMALL}
            onClick={() => handleCreatingChange(true)}
            isDisabled={state.isCreating || isSaving}
          />
          {!isLoading && (
            <Chip
              hasBorder
              isPill
              count={rows.length}
              size={ChipSize.SMALL}
              variant={ChipVariant.SECONDARY}
            />
          )}
        </Flex>
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
            {hasRows && <Divider />}
          </>
        )}
        {!hasRows && !state.isCreating && (
          <Box height={PIPELINE_NOTIFIER_TABLE_EMPTY_HEIGHT} fillWidth>
            <EmptyLayout
              size={EmptyLayoutSize.SMALL}
              header="No notifiers"
              description="Add a notifier to get notified when runs complete or fail."
            />
          </Box>
        )}
        {hasRows && (
          <Box variant={BoxVariant.PRIMARY} fillWidth>
            <InfiniteTable<TRow>
              ariaLabel={header}
              hasHeader={false}
              columns={columns}
              data={rows}
              getRowId={(row) => row.id}
              isLoading={isLoading}
              expandedIds={state.expandedRowIds}
              onExpandedIdsChange={handleExpandedChange}
              renderExpandedRow={(row) => (
                <PipelineNotifierForm
                  initialState={row}
                  isSaving={isSaving}
                  onSave={(next) => onUpdate(row, next, () => handleExpandedChange([]))}
                  onCancel={() => handleExpandedChange([])}
                  onDelete={() => onDelete(row)}
                />
              )}
            />
          </Box>
        )}
      </Flex>
    </Widget>
  );
};

export default PipelineNotifierTable;
