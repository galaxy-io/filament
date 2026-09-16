import FlexWrapper, { FlexDirection } from "@galaxy-io/dls/containers/FlexWrapper";
import { InputSize, InputVariant } from "@galaxy-io/dls/inputs/Input";
import SelectInput, { type SelectInputOption } from "@galaxy-io/dls/inputs/SelectInput";
import TextInput from "@galaxy-io/dls/inputs/TextInput";

import type { Resource, ResourceColumn } from "@/gen/ingestion/v1/connectors_pb";
import type { TransformFunction } from "@/gen/ingestion/v1/transformations_pb";

import { TRANSFORM_STEP_KIND_OPTIONS } from "@/pages/pipelines/canvas/panel/overview/resource/transform/constants";
import PipelineCanvasPanelResourceTransformExpression from "@/pages/pipelines/canvas/panel/overview/resource/transform/PipelineCanvasPanelResourceTransformExpression";
import {
  TransformStepKind,
  type TransformStepState,
} from "@/pages/pipelines/canvas/panel/overview/resource/transform/types";

interface PipelineCanvasPanelResourceTransformFieldsProps {
  state: TransformStepState;
  resources: Resource["name"][];
  columnsByResource: Map<Resource["name"], ResourceColumn[]>;
  functionsByName: Map<string, TransformFunction>;
  onChange: (partial: Partial<TransformStepState>) => void;
  isDisabled?: boolean;
}

const PipelineCanvasPanelResourceTransformFields = ({
  state,
  resources,
  columnsByResource,
  functionsByName,
  onChange,
  isDisabled = false,
}: PipelineCanvasPanelResourceTransformFieldsProps) => {
  const columns = columnsByResource.get(state.resource) ?? [];
  const column = columns.find((candidate) => candidate.name === state.column);

  const resourceOptions: SelectInputOption[] = resources.map((resource) => ({
    id: resource,
    label: resource,
    value: resource,
  }));
  const columnOptions: SelectInputOption[] = columns.map((candidate) => ({
    id: candidate.name,
    label: candidate.name,
    value: candidate.name,
  }));

  return (
    <FlexWrapper direction={FlexDirection.COLUMN} gap={12} fillWidth>
      {resources.length > 1 && (
        <SelectInput
          label="Resource"
          options={resourceOptions}
          value={resourceOptions.find((option) => option.id === state.resource) ?? null}
          onChange={(option) =>
            onChange({ resource: option.value as string, column: "", expression: [] })
          }
          variant={InputVariant.TERTIARY}
          placeholder="Choose a resource"
          size={InputSize.LARGE}
          isDisabled={isDisabled}
          fillWidth
        />
      )}
      <SelectInput
        label="Column"
        options={columnOptions}
        value={columnOptions.find((option) => option.id === state.column) ?? null}
        onChange={(option) => onChange({ column: option.value as string, expression: [] })}
        onSearch={(term, options) =>
          options.filter((option) => option.label.toLowerCase().includes(term.toLowerCase()))
        }
        variant={InputVariant.TERTIARY}
        placeholder={state.resource === "" ? "Choose a resource first" : "Choose a column"}
        size={InputSize.LARGE}
        isDisabled={isDisabled || state.resource === ""}
        fillWidth
      />
      <SelectInput
        label="Step"
        options={TRANSFORM_STEP_KIND_OPTIONS}
        value={TRANSFORM_STEP_KIND_OPTIONS.find((option) => option.id === state.kind) ?? null}
        onChange={(option) => onChange({ kind: option.value as TransformStepKind })}
        variant={InputVariant.TERTIARY}
        size={InputSize.LARGE}
        isDisabled={isDisabled}
        fillWidth
      />
      {state.kind === TransformStepKind.RENAME && (
        <TextInput
          label="Rename to"
          value={state.rename}
          onChange={(rename) => onChange({ rename })}
          placeholder={state.column === "" ? "" : `${state.column}_v2`}
          variant={InputVariant.TERTIARY}
          size={InputSize.LARGE}
          isDisabled={isDisabled}
          fillWidth
        />
      )}
      {state.kind === TransformStepKind.COMPUTE && (
        <>
          <PipelineCanvasPanelResourceTransformExpression
            columnType={column?.logicalType ?? ""}
            expression={state.expression}
            functionsByName={functionsByName}
            onChange={(expression) => onChange({ expression })}
            isDisabled={isDisabled || state.column === ""}
          />
          <TextInput
            label="As"
            value={state.output}
            onChange={(output) => onChange({ output })}
            placeholder="Same column, or a new name"
            variant={InputVariant.TERTIARY}
            size={InputSize.LARGE}
            isDisabled={isDisabled}
            fillWidth
          />
        </>
      )}
    </FlexWrapper>
  );
};

export default PipelineCanvasPanelResourceTransformFields;
