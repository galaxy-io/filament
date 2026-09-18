import { useMemo, useState } from "react";

import { create } from "@bufbuild/protobuf";
import { TrashIcon } from "@phosphor-icons/react";

import Button, { ButtonSize, ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import FlexWrapper, {
  AlignItems,
  FlexDirection,
  JustifyContent,
} from "@galaxy-io/dls/containers/FlexWrapper";
import { useDebouncedValue } from "@galaxy-io/dls/inputs/hooks";
import Text, { TextSize, TextVariant } from "@galaxy-io/dls/text/Text";
import Widget, { WidgetVariant } from "@galaxy-io/dls/widget/Widget";

import type { Connection } from "@/gen/ingestion/v1/connections_pb";
import type { Resource, ResourceColumn } from "@/gen/ingestion/v1/connectors_pb";
import {
  type TransformFunction,
  ValidateTransformRequestSchema,
} from "@/gen/ingestion/v1/transformations_pb";

import PipelineCanvasPanelResourceTransformFields from "@/pages/pipelines/canvas/panel/overview/resource/transform/PipelineCanvasPanelResourceTransformFields";
import {
  type TransformStep,
  TransformStepKind,
  type TransformStepState,
} from "@/pages/pipelines/canvas/panel/overview/resource/transform/types";
import {
  getTransformExpressionInfo,
  getTransformOutputName,
  getTransformStepValidationError,
  mapTransformStepsToDefinition,
} from "@/pages/pipelines/canvas/panel/overview/resource/transform/utils";

import { useValidateTransformQuery } from "@/api/queries/transforms";

interface PipelineCanvasPanelResourceTransformFormProps {
  initialState: TransformStepState;
  sourceConnectionId: Connection["id"];
  resources: Resource["name"][];
  columnsByResource: Map<Resource["name"], ResourceColumn[]>;
  functionsByName: Map<string, TransformFunction>;
  steps: TransformStep[];
  stepIndex: number;
  onSave: (state: TransformStepState) => void;
  onCancel: () => void;
  onDelete?: () => void;
  isDisabled?: boolean;
}

const isIdentityCompute = (state: TransformStepState): boolean => {
  if (state.kind !== TransformStepKind.COMPUTE || state.outputs.length !== 1) return false;
  const output = state.outputs[0];
  return (
    output.expression.source.kind === "column" &&
    output.expression.calls.length === 0 &&
    getTransformOutputName(output) === output.expression.source.column
  );
};

const PipelineCanvasPanelResourceTransformForm = ({
  initialState,
  sourceConnectionId,
  resources,
  columnsByResource,
  functionsByName,
  steps,
  stepIndex,
  onSave,
  onCancel,
  onDelete,
  isDisabled = false,
}: PipelineCanvasPanelResourceTransformFormProps) => {
  const [state, setState] = useState<TransformStepState>(initialState);
  const debouncedState = useDebouncedValue(state, 250);

  const handleChange = (partial: Partial<TransformStepState>) => {
    setState((prev) => ({ ...prev, ...partial }));
  };

  const originalPosition = Number.isFinite(stepIndex)
    ? steps.findIndex((step) => step.resource === initialState.resource && step.index === stepIndex)
    : -1;
  const effectiveStepIndex =
    Number.isFinite(stepIndex) && state.resource !== initialState.resource
      ? steps
          .slice(0, Math.max(0, originalPosition))
          .filter((step) => step.resource === state.resource).length
      : stepIndex;
  const prefixSteps = useMemo(
    () =>
      steps.filter((step) => step.resource === state.resource && step.index < effectiveStepIndex),
    [effectiveStepIndex, state.resource, steps],
  );
  const prefixDefinition = useMemo(() => mapTransformStepsToDefinition(prefixSteps), [prefixSteps]);
  const requiresPrefixValidation =
    sourceConnectionId !== "" && state.resource !== "" && prefixDefinition !== undefined;
  const prefixValidation = useValidateTransformQuery({
    input: create(ValidateTransformRequestSchema, {
      sourceConnectionId,
      resource: state.resource,
      transform: prefixDefinition,
    }),
    options: { enabled: requiresPrefixValidation },
  });
  const columns = prefixDefinition
    ? (prefixValidation.data?.outputColumns ?? [])
    : (columnsByResource.get(state.resource) ?? []);
  const prefixValidationError =
    prefixValidation.error !== null
      ? "Unable to load the columns available before this step."
      : prefixValidation.data && !prefixValidation.data.valid
        ? (prefixValidation.data.issues[0]?.message ??
          "An earlier transformation prevents this step from being edited.")
        : null;
  const isPrefixValidationPending =
    requiresPrefixValidation &&
    (prefixValidation.isFetching || prefixValidation.data === undefined);
  const selectedColumn =
    state.kind === TransformStepKind.RENAME
      ? state.renames[0]?.source
      : state.kind === TransformStepKind.DROP
        ? state.drops[0]
        : state.outputs[0]?.expression.source.kind === "column"
          ? state.outputs[0].expression.source.column
          : undefined;
  const selectedType = columns.find((column) => column.name === selectedColumn)?.logicalType ?? "";
  const firstExpression = state.outputs[0]?.expression;
  const outputInfo = firstExpression
    ? getTransformExpressionInfo(firstExpression, columns, functionsByName)
    : null;
  const typeSummary =
    state.kind === TransformStepKind.COMPUTE && outputInfo?.complete
      ? selectedType !== "" && selectedType !== outputInfo.type
        ? `${selectedType} → ${outputInfo.type}`
        : outputInfo.type
      : selectedType;
  const clientValidationError =
    getTransformStepValidationError(state, columns, functionsByName) ??
    (isIdentityCompute(state) && !isIdentityCompute(initialState)
      ? "Choose a function or save the result to another column."
      : null);
  const originalStep = Number.isFinite(stepIndex)
    ? steps.find((step) => step.resource === initialState.resource && step.index === stepIndex)
    : undefined;
  const draftSteps = useMemo(() => {
    if (originalStep) {
      return steps.map((step) =>
        step.id === originalStep.id ? { ...step, ...debouncedState } : step,
      );
    }
    return [
      ...steps,
      {
        ...debouncedState,
        id: "draft",
        index: steps.filter((step) => step.resource === debouncedState.resource).length,
      },
    ];
  }, [debouncedState, originalStep, steps]);
  const draftDefinition = useMemo(() => mapTransformStepsToDefinition(draftSteps), [draftSteps]);
  const isAwaitingDraftValidation = debouncedState !== state;
  const requiresDraftValidation =
    sourceConnectionId !== "" && state.resource !== "" && draftDefinition !== undefined;
  const draftValidation = useValidateTransformQuery({
    input: create(ValidateTransformRequestSchema, {
      sourceConnectionId,
      resource: state.resource,
      transform: draftDefinition,
    }),
    options: {
      enabled:
        requiresDraftValidation &&
        !isAwaitingDraftValidation &&
        !isPrefixValidationPending &&
        prefixValidationError === null &&
        clientValidationError === null,
    },
  });
  const draftValidationError = isAwaitingDraftValidation
    ? null
    : draftValidation.error !== null
      ? "Unable to validate this transformation."
      : draftValidation.data && !draftValidation.data.valid
        ? (() => {
            const issue = draftValidation.data.issues[0];
            if (!issue) return "This transformation is not valid.";
            const stepMatch = /\.steps\[(\d+)\]/.exec(issue.field);
            return stepMatch ? `Step ${Number(stepMatch[1]) + 1}: ${issue.message}` : issue.message;
          })()
        : null;
  const isDraftValidationPending =
    requiresDraftValidation &&
    clientValidationError === null &&
    (isAwaitingDraftValidation || draftValidation.isFetching || draftValidation.data === undefined);
  const isSaveDisabled =
    isDisabled ||
    clientValidationError !== null ||
    prefixValidationError !== null ||
    isPrefixValidationPending ||
    draftValidationError !== null ||
    isDraftValidationPending;

  return (
    <FlexWrapper padding="12px" fillWidth>
      {/* The section body is tertiary, so the card matches it and only its border shows, as the notifier card does on its primary container. */}
      <Widget variant={WidgetVariant.TERTIARY} noHover padding="16px" fillWidth>
        <FlexWrapper direction={FlexDirection.COLUMN} gap={12} fillWidth>
          <PipelineCanvasPanelResourceTransformFields
            state={state}
            resources={resources}
            columns={columns}
            functionsByName={functionsByName}
            onChange={handleChange}
            isDisabled={isDisabled || isPrefixValidationPending || prefixValidationError !== null}
          />
          {(prefixValidationError ?? draftValidationError) && (
            <Text size={TextSize.BODY_SM} variant={TextVariant.ERROR}>
              {prefixValidationError ?? draftValidationError}
            </Text>
          )}
          <FlexWrapper
            alignItems={AlignItems.CENTER}
            justifyContent={JustifyContent.SPACE_BETWEEN}
            gap={8}
            fillWidth
          >
            <Text size={TextSize.BODY_SM} variant={TextVariant.SECONDARY} isMonospace isEllipsis>
              {typeSummary}
            </Text>
            <FlexWrapper alignItems={AlignItems.CENTER} gap={8} shrink={0}>
              {onDelete && (
                <Button
                  label="Delete"
                  icon={TrashIcon}
                  variant={ButtonVariant.SECONDARY}
                  size={ButtonSize.MEDIUM}
                  onClick={onDelete}
                  isDisabled={isDisabled}
                  ariaLabel="Delete step"
                />
              )}
              <Button
                label="Cancel"
                variant={ButtonVariant.SECONDARY}
                size={ButtonSize.MEDIUM}
                onClick={onCancel}
                isDisabled={isDisabled}
              />
              <Button
                label="Save"
                size={ButtonSize.MEDIUM}
                onClick={() => onSave(state)}
                isDisabled={isSaveDisabled}
              />
            </FlexWrapper>
          </FlexWrapper>
        </FlexWrapper>
      </Widget>
    </FlexWrapper>
  );
};

export default PipelineCanvasPanelResourceTransformForm;
