import { useState } from "react";

import Button, { ButtonSize, ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import { InputSize, InputVariant } from "@galaxy-io/dls/inputs/Input";
import SelectInput, {
  SelectInputSize,
  SelectInputVariant,
  type SelectOption,
} from "@galaxy-io/dls/inputs/SelectInput";
import TextInput from "@galaxy-io/dls/inputs/TextInput";
import Flex, { AlignItems, FlexDirection, JustifyContent } from "@galaxy-io/dls/layout/Flex";
import Text, { TextSize, TextVariant } from "@galaxy-io/dls/text/Text";
import Widget, { WidgetVariant } from "@galaxy-io/dls/widget/Widget";

import type { Resource } from "@/gen/ingestion/v1/connectors_pb";

import type { CanvasNode } from "@/pages/pipelines/canvas/types";

export interface PipelineResourceCreateSink {
  id: CanvasNode["id"];
  label: string;
}

export interface PipelineResourceCreateState {
  resource: Resource["name"];
  sinkId: CanvasNode["id"];
}

interface PipelineResourceCreateFormProps {
  sinks: PipelineResourceCreateSink[];
  getError: (state: PipelineResourceCreateState) => string | null;
  onSave: (state: PipelineResourceCreateState) => void;
  onCancel: () => void;
}

const PipelineResourceCreateForm = ({
  sinks,
  getError,
  onSave,
  onCancel,
}: PipelineResourceCreateFormProps) => {
  const [state, setState] = useState<PipelineResourceCreateState>({
    resource: "",
    sinkId: sinks[0]?.id ?? "",
  });

  const trimmed = { ...state, resource: state.resource.trim() };
  const error = trimmed.resource === "" ? null : getError(trimmed);
  const isSaveDisabled = trimmed.resource === "" || trimmed.sinkId === "" || error !== null;

  const sinkOptions: SelectOption[] = sinks.map((sink) => ({
    id: sink.id,
    label: sink.label,
    value: sink.id,
  }));

  return (
    <Flex alignItems={AlignItems.START} padding={12} fillWidth>
      <Widget
        variant={
          WidgetVariant.TERTIARY
        } /* @dls-migrate widget.padding-other: The body inset is fixed at 12px: remove `padding` (use `isFlush` for 0). */
        padding="16px" /* @dls-migrate widget.fillWidth: Grow the card with a `FlexItem` or a `Grid` track. */
        fillWidth
      >
        <Flex alignItems={AlignItems.START} direction={FlexDirection.COLUMN} gap={12} fillWidth>
          <TextInput
            label="Resource"
            value={state.resource}
            onChange={(resource) => setState((prev) => ({ ...prev, resource }))}
            onKeyDown={(event) => {
              if (event.key === "Enter" && !isSaveDisabled) onSave(trimmed);
            }}
            placeholder="orders.>"
            variant={InputVariant.TERTIARY}
            size={InputSize.LARGE}
            autoFocus
            fillWidth
          />
          {sinks.length > 1 && (
            <SelectInput
              label="Sink"
              options={sinkOptions}
              /* @dls-migrate selectinput.value: `value` and `onChange` now carry option ids, not option objects. */ value={
                sinkOptions.find((option) => option.value === state.sinkId) ?? null
              }
              onChange={(option) =>
                setState((prev) => ({ ...prev, sinkId: option.value as string }))
              }
              variant={SelectInputVariant.TERTIARY}
              size={SelectInputSize.LARGE}
              fillWidth
            />
          )}
          {error && (
            <Text size={TextSize.BODY_SM} variant={TextVariant.ERROR}>
              {error}
            </Text>
          )}
          <Flex
            alignItems={AlignItems.CENTER}
            justifyContent={JustifyContent.END}
            gap={8}
            fillWidth
          >
            <Button
              label="Cancel"
              variant={ButtonVariant.SECONDARY}
              size={ButtonSize.MEDIUM}
              onClick={onCancel}
            />
            <Button
              label="Add"
              size={ButtonSize.MEDIUM}
              onClick={() => onSave(trimmed)}
              isDisabled={isSaveDisabled}
            />
          </Flex>
        </Flex>
      </Widget>
    </Flex>
  );
};

export default PipelineResourceCreateForm;
