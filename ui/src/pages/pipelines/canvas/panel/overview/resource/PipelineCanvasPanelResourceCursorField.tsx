import FlexWrapper, { AlignItems, JustifyContent } from "@galaxy-io/dls/containers/FlexWrapper";
import { InputSize } from "@galaxy-io/dls/inputs/Input";
import SelectInput, { type SelectInputOption } from "@galaxy-io/dls/inputs/SelectInput";
import Text, { TextSize, TextVariant } from "@galaxy-io/dls/text/Text";

import type { Resource, ResourceColumn } from "@/gen/ingestion/v1/connectors_pb";

interface PipelineCanvasPanelResourceCursorFieldProps {
  resource: Resource["name"];
  value: ResourceColumn["name"];
  options: ResourceColumn[];
  isDisabled: boolean;
  onChange: (field: ResourceColumn["name"]) => void;
}

const PipelineCanvasPanelResourceCursorField = ({
  resource,
  value,
  options,
  isDisabled,
  onChange,
}: PipelineCanvasPanelResourceCursorFieldProps) => {
  if (!options.length) {
    return (
      <FlexWrapper
        alignItems={AlignItems.CENTER}
        justifyContent={JustifyContent.SPACE_BETWEEN}
        gap={8}
      >
        <Text size={TextSize.BODY_SM} isEllipsis>
          {resource}
        </Text>
        <Text size={TextSize.BODY_SM} variant={TextVariant.TERTIARY}>
          Auto
        </Text>
      </FlexWrapper>
    );
  }

  const selectOptions: SelectInputOption[] = options.map((column) => ({
    id: column.name,
    label: column.name,
    value: column.name,
  }));

  return (
    <SelectInput
      label="Cursor"
      options={selectOptions}
      value={selectOptions.find((option) => option.value === value) ?? null}
      onChange={(option) => onChange(option.value as string)}
      placeholder="Select a column..."
      size={InputSize.LARGE}
      isDisabled={isDisabled}
      fillWidth
    />
  );
};

export default PipelineCanvasPanelResourceCursorField;
