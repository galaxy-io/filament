import { useState } from "react";

import { PlusIcon } from "@phosphor-icons/react";

import Accordion, { type AccordionSize } from "@galaxy-io/dls/accordion/Accordion";
import Button, { ButtonSize, ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import FlexWrapper, { FlexDirection } from "@galaxy-io/dls/containers/FlexWrapper";
import HorizontalDivider from "@galaxy-io/dls/dividers/HorizontalDivider";
import { InputSize } from "@galaxy-io/dls/inputs/Input";
import ToggleInput from "@galaxy-io/dls/inputs/ToggleInput";
import InfiniteTable, {
  ColumnAlign,
  type ColumnDef,
  TableVariant,
} from "@galaxy-io/dls/table/InfiniteTable";
import Text, { TextSize } from "@galaxy-io/dls/text/Text";
import TextShimmer from "@galaxy-io/dls/text/TextShimmer";

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
  size?: AccordionSize;
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

  const columns: ColumnDef<TRow>[] = [
    {
      id: "name",
      header: "Name",
      cellLoading: () => <TextShimmer width={140} height={16} />,
      cell: ({ row }) => (
        <Text size={TextSize.BODY_SM} isEllipsis>
          {row.original.name}
        </Text>
      ),
    },
    {
      id: "enabled",
      header: "",
      align: ColumnAlign.RIGHT,
      size: PIPELINE_NOTIFIER_TABLE_COLUMN_WIDTH_ENABLED,
      cellLoading: () => null,
      cell: ({ row }) => (
        <ToggleInput
          size={InputSize.LARGE}
          value={row.original.isEnabled}
          onChange={(isEnabled) => onToggleEnabled(row.original, isEnabled)}
          isDisabled={isSaving}
        />
      ),
    },
  ];

  return (
    <Accordion
      header={header}
      size={size}
      padding={0}
      isOpen={state.isOpen}
      onToggle={handleToggle}
      trailing={
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
      <FlexWrapper direction={FlexDirection.COLUMN} fillWidth>
        {state.isCreating && (
          <>
            <PipelineNotifierForm
              initialState={PIPELINE_NOTIFIER_DEFAULT_STATE}
              isSaving={isSaving}
              onSave={(next) => onCreate(next, () => handleCreatingChange(false))}
              onCancel={() => handleCreatingChange(false)}
            />
            <HorizontalDivider />
          </>
        )}
        <InfiniteTable<TRow>
          columns={columns}
          data={rows}
          getRowId={(row) => row.id}
          isLoading={isLoading}
          contentWhenEmpty={
            <EmptyLayout
              size={LayoutSize.SMALL}
              header="No notifiers"
              message="Add a notifier to get notified when runs complete or fail."
            />
          }
          expandedRowIds={state.expandedRowIds}
          onExpandedChange={handleExpandedChange}
          onRowExpand={(row) => (
            <PipelineNotifierForm
              initialState={row.original}
              isSaving={isSaving}
              onSave={(next) => onUpdate(row.original, next, () => handleExpandedChange([]))}
              onCancel={() => handleExpandedChange([])}
              onDelete={() => onDelete(row.original)}
            />
          )}
          enableMultiRowExpansion={false}
          variant={TableVariant.PRIMARY}
          noLastRowBorder
          noLastRowPadding
          noHeader
          fillWidth
        />
      </FlexWrapper>
    </Accordion>
  );
};

export default PipelineNotifierTable;
