import FlexWrapper, { AlignItems } from "@galaxy-io/dls/containers/FlexWrapper";
import RadioInput from "@galaxy-io/dls/inputs/RadioInput";
import Text, { TextSize, TextVariant } from "@galaxy-io/dls/text/Text";

import { ExecutionMode } from "@/gen/ingestion/v1/common_pb";

const options = [
  {
    label: "Batch",
    description: "Finish after reading available data.",
    value: ExecutionMode.BOUNDED,
  },
  {
    label: "Continuous",
    description: "Keep listening for new data until stopped.",
    value: ExecutionMode.CONTINUOUS,
  },
];

export default function PipelineExecutionModeField({
  value,
  onChange,
  isDisabled = false,
  supportedModes,
}: {
  value: ExecutionMode;
  onChange: (mode: ExecutionMode) => void;
  isDisabled?: boolean;
  supportedModes?: ExecutionMode[];
}) {
  return (
    <FlexWrapper alignItems={AlignItems.CENTER} gap={24} fillWidth>
      <Text size={TextSize.BODY_SM} variant={TextVariant.SECONDARY}>
        Run type
      </Text>
      {options.map((option) => {
        const unsupported = supportedModes !== undefined && !supportedModes.includes(option.value);
        return (
          <RadioInput
            key={option.value}
            label={option.label}
            labelTooltip={unsupported ? "Unavailable for these connections" : option.description}
            isSelected={value === option.value}
            isDisabled={isDisabled || unsupported}
            onChange={() => onChange(option.value)}
          />
        );
      })}
    </FlexWrapper>
  );
}
