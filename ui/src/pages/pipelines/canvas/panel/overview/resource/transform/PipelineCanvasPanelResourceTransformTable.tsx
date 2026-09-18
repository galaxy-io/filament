import { useEffect, useState } from "react";

import FlexWrapper, { FlexDirection } from "@galaxy-io/dls/containers/FlexWrapper";
import HorizontalDivider from "@galaxy-io/dls/dividers/HorizontalDivider";
import TextInput from "@galaxy-io/dls/inputs/TextInput";
import InfiniteTable, { type ColumnDef, TableVariant } from "@galaxy-io/dls/table/InfiniteTable";
import Text, { TextSize, TextVariant } from "@galaxy-io/dls/text/Text";

import type { Resource, ResourceColumn } from "@/gen/ingestion/v1/connectors_pb";
import type { TransformFunction } from "@/gen/ingestion/v1/transformations_pb";

import { createTransformStepDefaultState } from "@/pages/pipelines/canvas/panel/overview/resource/transform/constants";
import PipelineCanvasPanelResourceTransformForm from "@/pages/pipelines/canvas/panel/overview/resource/transform/PipelineCanvasPanelResourceTransformForm";
import type {
  TransformStep,
  TransformStepState,
} from "@/pages/pipelines/canvas/panel/overview/resource/transform/types";
import { formatTransformStepSummary } from "@/pages/pipelines/canvas/panel/overview/resource/transform/utils";

interface PipelineCanvasPanelResourceTransformTableProps {
  steps: TransformStep[];
  resources: Resource["name"][];
  columnsByResource: Map<Resource["name"], ResourceColumn[]>;
  functionsByName: Map<string, TransformFunction>;
  onCreate: (state: TransformStepState) => void;
  onUpdate: (step: TransformStep, state: TransformStepState) => void;
  onDelete: (step: TransformStep) => void;
  isCreating: boolean;
  onCreatingChange: (isCreating: boolean) => void;
  isReadOnly?: boolean;
}

/**
 * The steps of an edge, one row per step in definition order. Rows expand
 * into the form; a step the builder cannot represent is shown but not edited.
 */
const PipelineCanvasPanelResourceTransformTable = ({
  steps,
  resources,
  columnsByResource,
  functionsByName,
  onCreate,
  onUpdate,
  onDelete,
  isCreating,
  onCreatingChange,
  isReadOnly = false,
}: PipelineCanvasPanelResourceTransformTableProps) => {
  const [expandedRowIds, setExpandedRowIds] = useState<TransformStep["id"][]>([]);
  const [search, setSearch] = useState("");

  const handleExpandedChange = (next: TransformStep["id"][]) => {
    setExpandedRowIds(next.filter((id) => steps.find((step) => step.id === id)?.raw === undefined));
  };

  // Add step lives in the section header; starting a step closes the open row.
  useEffect(() => {
    if (isCreating) {
      setExpandedRowIds([]);
    }
  }, [isCreating]);

  const columns: ColumnDef<TransformStep>[] = [
    {
      id: "step",
      header: "Step",
      cell: ({ row }) => (
        <FlexWrapper direction={FlexDirection.COLUMN} gap={2} minWidth={0}>
          <Text size={TextSize.BODY_MD} isMonospace={row.original.raw === undefined} isEllipsis>
            {formatTransformStepSummary(row.original)}
          </Text>
          {resources.length > 1 && (
            <Text size={TextSize.CAPTION} variant={TextVariant.SECONDARY} isEllipsis>
              {row.original.resource}
            </Text>
          )}
        </FlexWrapper>
      ),
    },
  ];

  const defaultState: TransformStepState = createTransformStepDefaultState(
    resources.length === 1 ? resources[0] : "",
  );
  const normalizedSearch = search.trim().toLowerCase();
  const filteredSteps =
    normalizedSearch === ""
      ? steps
      : steps.filter((step) =>
          `${formatTransformStepSummary(step)} ${step.resource}`
            .toLowerCase()
            .includes(normalizedSearch),
        );

  return (
    <FlexWrapper direction={FlexDirection.COLUMN} fillWidth>
      {steps.length > 0 && (
        <>
          <FlexWrapper padding="8px" fillWidth>
            <TextInput placeholder="Search" value={search} onChange={setSearch} fillWidth />
          </FlexWrapper>
          <HorizontalDivider />
        </>
      )}
      {isCreating && (
        <>
          <PipelineCanvasPanelResourceTransformForm
            initialState={defaultState}
            resources={resources}
            columnsByResource={columnsByResource}
            functionsByName={functionsByName}
            steps={steps}
            stepIndex={Number.POSITIVE_INFINITY}
            onSave={(next) => {
              onCreate(next);
              onCreatingChange(false);
            }}
            onCancel={() => onCreatingChange(false)}
          />
          {steps.length > 0 && <HorizontalDivider />}
        </>
      )}
      {steps.length > 0 && (
        <InfiniteTable<TransformStep>
          columns={columns}
          data={filteredSteps}
          getRowId={(step) => step.id}
          expandedRowIds={expandedRowIds}
          onExpandedChange={isReadOnly ? undefined : handleExpandedChange}
          onRowExpand={(row) => (
            <PipelineCanvasPanelResourceTransformForm
              initialState={row.original}
              resources={resources}
              columnsByResource={columnsByResource}
              functionsByName={functionsByName}
              steps={steps}
              stepIndex={row.original.index}
              onSave={(next) => {
                onUpdate(row.original, next);
                handleExpandedChange([]);
              }}
              onCancel={() => handleExpandedChange([])}
              onDelete={() => {
                onDelete(row.original);
                handleExpandedChange([]);
              }}
            />
          )}
          enableMultiRowExpansion={false}
          variant={TableVariant.PRIMARY}
          noLastRowBorder
          noLastRowPadding
          noHeader
          fillWidth
        />
      )}
    </FlexWrapper>
  );
};

export default PipelineCanvasPanelResourceTransformTable;
