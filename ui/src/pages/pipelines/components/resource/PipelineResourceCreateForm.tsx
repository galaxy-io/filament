import { type FC, useState } from "react";

import Button, { ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import { InputVariant } from "@galaxy-io/dls/inputs/Input";
import SelectInput, {
  SelectInputVariant,
  type SelectOption,
} from "@galaxy-io/dls/inputs/SelectInput";
import TextInput from "@galaxy-io/dls/inputs/TextInput";
import Flex, { AlignItems, FlexDirection, JustifyContent } from "@galaxy-io/dls/layout/Flex";
import Widget, { WidgetSize, WidgetVariant } from "@galaxy-io/dls/widget/Widget";

import type { CanvasNode } from "@/pages/pipelines/canvas/types";
import type { PipelineResourceCreateState } from "@/pages/pipelines/components/resource/types";

interface PipelineResourceCreateSink {
  id: CanvasNode["id"];
  label: string;
}

interface PipelineResourceCreateFormProps {
  sinks: PipelineResourceCreateSink[];
  getError: (state: PipelineResourceCreateState) => string | undefined;
  onSave: (state: PipelineResourceCreateState) => void;
  onCancel: () => void;
  variant?: WidgetVariant;
}

const DEFAULT_STATE: PipelineResourceCreateState = {
  resource: "",
  sinkId: "",
};

const PipelineResourceCreateForm: FC<PipelineResourceCreateFormProps> = ({
  sinks,
  getError,
  onSave,
  onCancel,
  variant = WidgetVariant.TERTIARY,
}) => {
  const [state, setState] = useState<PipelineResourceCreateState>(() => ({
    ...DEFAULT_STATE,
    sinkId: sinks[0]?.id ?? DEFAULT_STATE.sinkId,
  }));

  const trimmed = { ...state, resource: state.resource.trim() };
  const error = trimmed.resource === "" ? undefined : getError(trimmed);
  const isSaveDisabled = trimmed.resource === "" || trimmed.sinkId === "" || error !== undefined;

  const sinkOptions: SelectOption[] = sinks.map((sink) => ({
    id: sink.id,
    label: sink.label,
  }));

  return (
    <Flex alignItems={AlignItems.STRETCH} direction={FlexDirection.COLUMN} padding={12} fillWidth>
      <Widget variant={variant} size={WidgetSize.LARGE} gap={12}>
        <TextInput
          label="Resource"
          value={state.resource}
          onChange={(resource) => setState((prev) => ({ ...prev, resource }))}
          onKeyDown={(event) => {
            if (event.key === "Enter" && !isSaveDisabled) onSave(trimmed);
          }}
          placeholder="orders.>"
          error={error}
          autoFocus
          variant={InputVariant.TERTIARY}
          fillWidth
        />
        {sinks.length > 1 && (
          <SelectInput
            label="Sink"
            options={sinkOptions}
            value={state.sinkId || null}
            onChange={(sinkId) => {
              if (sinkId !== null) setState((prev) => ({ ...prev, sinkId }));
            }}
            variant={SelectInputVariant.TERTIARY}
            fillWidth
          />
        )}
        <Flex alignItems={AlignItems.CENTER} justifyContent={JustifyContent.END} gap={8} fillWidth>
          <Button label="Cancel" variant={ButtonVariant.SECONDARY} onClick={onCancel} />
          <Button
            label="Add resource"
            onClick={() => onSave(trimmed)}
            isDisabled={isSaveDisabled}
          />
        </Flex>
      </Widget>
    </Flex>
  );
};

export default PipelineResourceCreateForm;
