import FlexWrapper, {
  AlignItems,
  FlexDirection,
} from "@galaxy-io/dls/containers/FlexWrapper";
import ToggleInput from "@galaxy-io/dls/inputs/ToggleInput";
import RequiredMarker from "@galaxy-io/dls/text/RequiredMarker";
import Text, { TextSize, TextVariant } from "@galaxy-io/dls/text/Text";

import type { FieldComponentProps } from "./types";
import HelpIcon from "@galaxy-io/dls/icons/HelpIcon";
import Spacing from "@galaxy-io/dls/containers/Spacing";
import Tooltip, { TooltipPosition } from "@galaxy-io/dls/tooltip/Tooltip";

const BoolField = ({
  field,
  value,
  onChange,
  error,
  isDisabled = false,
  label,
}: FieldComponentProps) => {
  return (
    <FlexWrapper direction={FlexDirection.COLUMN} gap={6} fillWidth>
      <FlexWrapper alignItems={AlignItems.CENTER} gap={1}>
        <Text variant={TextVariant.SECONDARY}>{label}</Text>
        {field.required && <RequiredMarker />}
        {field.help && (
          <Spacing left={4}>
            <Tooltip body={field.help} position={TooltipPosition.RIGHT}>
              <HelpIcon />
            </Tooltip>
          </Spacing>
        )}
      </FlexWrapper>
      <FlexWrapper direction={FlexDirection.COLUMN} gap={2} fillWidth>
        {error && (
          <Text size={TextSize.CAPTION} variant={TextVariant.ERROR}>
            {error}
          </Text>
        )}
      </FlexWrapper>
      <ToggleInput
        value={(value as boolean) ?? false}
        onChange={(v) => onChange(v)}
        isDisabled={isDisabled}
      />
    </FlexWrapper>
  );
};

export default BoolField;
