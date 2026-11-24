import type { FC } from "react";

import type { JsonValue } from "@bufbuild/protobuf";

import SelectInput, {
  SelectInputVariant,
  type SelectOption,
} from "@galaxy-io/dls/inputs/SelectInput";
import Box from "@galaxy-io/dls/layout/Box";

import type { ConfigField } from "@/gen/ingestion/v1/common_pb";

import { PIPELINE_RESOURCE_SINK_FIELD_UNSET_ID } from "@/pages/pipelines/components/resource/constants";

interface PipelineResourceSinkFieldEnumProps {
  field: ConfigField;
  value: JsonValue | undefined;
  pipelineValue: JsonValue | undefined;
  label: string;
  ariaLabel: string;
  isCompact: boolean;
  isDisabled: boolean;
  isReadOnly: boolean;
  onChange: (value: JsonValue | undefined) => void;
}

const PipelineResourceSinkFieldEnum: FC<PipelineResourceSinkFieldEnumProps> = ({
  field,
  value,
  pipelineValue,
  label,
  ariaLabel,
  isCompact,
  isDisabled,
  isReadOnly,
  onChange,
}) => {
  const hasPipelineValue = typeof pipelineValue === "string";
  const options: SelectOption[] = [
    ...(hasPipelineValue ? [] : [{ id: PIPELINE_RESOURCE_SINK_FIELD_UNSET_ID, label: "Not set" }]),
    ...field.enum.map((option) => ({ id: option.value, label: option.label || option.value })),
  ];
  const effective = typeof value === "string" ? value : pipelineValue;

  const handleChange = (id: string | null) => {
    if (id === null) return;
    onChange(id === PIPELINE_RESOURCE_SINK_FIELD_UNSET_ID || id === pipelineValue ? undefined : id);
  };

  return (
    <Box fillWidth minWidth={0}>
      <SelectInput
        options={options}
        value={typeof effective === "string" ? effective : PIPELINE_RESOURCE_SINK_FIELD_UNSET_ID}
        onChange={handleChange}
        label={isCompact ? undefined : label}
        labelTooltip={isCompact ? undefined : field.help || undefined}
        variant={isCompact ? SelectInputVariant.BASE : SelectInputVariant.TERTIARY}
        isDisabled={isDisabled}
        isReadOnly={isReadOnly}
        ariaLabel={ariaLabel}
        fillWidth
      />
    </Box>
  );
};

export default PipelineResourceSinkFieldEnum;
