import { useState } from "react";

import { InfoIcon, TrashIcon } from "@phosphor-icons/react";

import Button, { ButtonSize, ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import FlexWrapper, {
  AlignItems,
  FlexDirection,
  JustifyContent,
} from "@galaxy-io/dls/containers/FlexWrapper";
import Icon, { IconVariant } from "@galaxy-io/dls/icons/Icon";
import Text, { TextSize, TextVariant } from "@galaxy-io/dls/text/Text";
import Tooltip from "@galaxy-io/dls/tooltip/Tooltip";
import Widget, { WidgetVariant } from "@galaxy-io/dls/widget/Widget";

import type { Resource, ResourceColumn } from "@/gen/ingestion/v1/connectors_pb";
import type { TransformFunction } from "@/gen/ingestion/v1/transformations_pb";

import PipelineCanvasPanelResourceTransformFields from "@/pages/pipelines/canvas/panel/overview/resource/transform/PipelineCanvasPanelResourceTransformFields";
import {
  TransformStepKind,
  type TransformStepState,
} from "@/pages/pipelines/canvas/panel/overview/resource/transform/types";
import {
  getTransformExpressionType,
  isTransformStepValid,
} from "@/pages/pipelines/canvas/panel/overview/resource/transform/utils";

interface PipelineCanvasPanelResourceTransformFormProps {
  initialState: TransformStepState;
  resources: Resource["name"][];
  columnsByResource: Map<Resource["name"], ResourceColumn[]>;
  functionsByName: Map<string, TransformFunction>;
  onSave: (state: TransformStepState) => void;
  onCancel: () => void;
  onDelete?: () => void;
  isDisabled?: boolean;
}

const PipelineCanvasPanelResourceTransformForm = ({
  initialState,
  resources,
  columnsByResource,
  functionsByName,
  onSave,
  onCancel,
  onDelete,
  isDisabled = false,
}: PipelineCanvasPanelResourceTransformFormProps) => {
  const [state, setState] = useState<TransformStepState>(initialState);

  const handleChange = (partial: Partial<TransformStepState>) => {
    setState((prev) => ({ ...prev, ...partial }));
  };

  const columnType =
    columnsByResource.get(state.resource)?.find((column) => column.name === state.column)
      ?.logicalType ?? "";
  const outputType =
    state.kind === TransformStepKind.COMPUTE && state.expression.length > 0
      ? getTransformExpressionType(columnType, state.expression, functionsByName)
      : columnType;
  // A drop has no output, so nothing to say; a rename or compute shows what the
  // column is, and what it becomes when the expression changes its type.
  const typeFlow =
    columnType === "" || state.kind === TransformStepKind.DROP
      ? ""
      : outputType !== "" && outputType !== columnType
        ? `${columnType} → ${outputType}`
        : columnType;

  return (
    <FlexWrapper padding="12px" fillWidth>
      {/* The section body is tertiary, so the card matches it and only its border shows, as the notifier card does on its primary container. */}
      <Widget variant={WidgetVariant.TERTIARY} noHover padding="16px" fillWidth>
        <FlexWrapper direction={FlexDirection.COLUMN} gap={12} fillWidth>
          <PipelineCanvasPanelResourceTransformFields
            state={state}
            resources={resources}
            columnsByResource={columnsByResource}
            functionsByName={functionsByName}
            onChange={handleChange}
            isDisabled={isDisabled}
          />
          <FlexWrapper
            alignItems={AlignItems.CENTER}
            justifyContent={JustifyContent.SPACE_BETWEEN}
            gap={8}
            fillWidth
          >
            {state.kind === TransformStepKind.DROP ? (
              <Tooltip body="The column is dropped before rows reach the sink.">
                <Icon component={InfoIcon} variant={IconVariant.TERTIARY} size={16} />
              </Tooltip>
            ) : (
              <Text size={TextSize.BODY_SM} variant={TextVariant.SECONDARY} isMonospace isEllipsis>
                {typeFlow}
              </Text>
            )}
            <FlexWrapper alignItems={AlignItems.CENTER} gap={8} shrink={0}>
              {onDelete && (
                <Button
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
                isDisabled={isDisabled || !isTransformStepValid(state)}
              />
            </FlexWrapper>
          </FlexWrapper>
        </FlexWrapper>
      </Widget>
    </FlexWrapper>
  );
};

export default PipelineCanvasPanelResourceTransformForm;
