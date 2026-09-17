import { useState } from "react";

import { PlusIcon } from "@phosphor-icons/react";

import Button, { ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import FlexItem from "@galaxy-io/dls/containers/FlexItem";
import FlexWrapper, { AlignItems, FlexDirection } from "@galaxy-io/dls/containers/FlexWrapper";
import HorizontalDivider from "@galaxy-io/dls/dividers/HorizontalDivider";
import { InputSize } from "@galaxy-io/dls/inputs/Input";
import TextInput from "@galaxy-io/dls/inputs/TextInput";
import ToggleInput from "@galaxy-io/dls/inputs/ToggleInput";
import InfiniteTable, {
  ColumnAlign,
  type ColumnDef,
  TableVariant,
} from "@galaxy-io/dls/table/InfiniteTable";
import Text, { TextSize } from "@galaxy-io/dls/text/Text";
import TextShimmer from "@galaxy-io/dls/text/TextShimmer";
import Widget from "@galaxy-io/dls/widget/Widget";

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

import { isSearchMatch } from "@/utils/search";

interface PipelineNotifierTableProps<TRow extends PipelineNotifier> {
  rows: TRow[];
  isLoading?: boolean;
  isSaving?: boolean;
  onCreate: (state: PipelineNotifierState, onSuccess: () => void) => void;
  onUpdate: (row: TRow, state: PipelineNotifierState, onSuccess: () => void) => void;
  onDelete: (row: TRow) => void;
  onToggleEnabled: (row: TRow, isEnabled: boolean) => void;
}

interface PipelineNotifierTableState {
  search: string;
  isCreating: boolean;
  expandedRowIds: PipelineNotifier["id"][];
}

const DEFAULT_STATE: PipelineNotifierTableState = {
  search: "",
  isCreating: false,
  expandedRowIds: [],
};

const PipelineNotifierTable = <TRow extends PipelineNotifier>({
  rows,
  isLoading = false,
  isSaving = false,
  onCreate,
  onUpdate,
  onDelete,
  onToggleEnabled,
}: PipelineNotifierTableProps<TRow>) => {
  const [state, setState] = useState<PipelineNotifierTableState>(DEFAULT_STATE);

  const filteredRows = rows.filter((row) => isSearchMatch(state.search, row.name));

  const handleSearchChange = (search: string) => {
    setState((prev) => ({ ...prev, search }));
  };

  const handleCreatingChange = (isCreating: boolean) => {
    setState((prev) => ({ ...prev, isCreating }));
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
    <Widget noPadding noHover fillWidth>
      <FlexWrapper direction={FlexDirection.COLUMN} fillWidth>
        <FlexWrapper alignItems={AlignItems.CENTER} gap={8} padding="8px" fillWidth>
          <TextInput
            placeholder="Search"
            value={state.search}
            onChange={handleSearchChange}
            fillWidth
          />
          <FlexItem shrink={0}>
            <Button
              label="Add notifier"
              icon={PlusIcon}
              variant={ButtonVariant.SECONDARY}
              onClick={() => handleCreatingChange(true)}
              isDisabled={state.isCreating || isSaving}
            />
          </FlexItem>
        </FlexWrapper>
        <HorizontalDivider />
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
          data={filteredRows}
          getRowId={(row) => row.id}
          isLoading={isLoading}
          contentWhenEmpty={
            <EmptyLayout
              size={LayoutSize.SMALL}
              header={rows.length === 0 ? "No notifiers" : undefined}
              message={
                rows.length === 0
                  ? "Add a notifier to get notified when runs complete or fail."
                  : "No notifiers match your search."
              }
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
    </Widget>
  );
};

export default PipelineNotifierTable;
