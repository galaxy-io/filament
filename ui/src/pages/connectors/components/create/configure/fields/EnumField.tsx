import { useState } from "react";

import FlexWrapper, {
  AlignItems,
  FlexDirection,
} from "@galaxy-io/dls/containers/FlexWrapper";
import Dropdown, { DropdownPosition } from "@galaxy-io/dls/dropdown/Dropdown";
import DropdownButton from "@galaxy-io/dls/dropdown/DropdownButton";
import DropdownItem from "@galaxy-io/dls/dropdown/DropdownItem";
import RequiredMarker from "@galaxy-io/dls/text/RequiredMarker";
import Text, { TextSize, TextVariant } from "@galaxy-io/dls/text/Text";

import type { FieldComponentProps } from "./types";
import Tooltip from "@galaxy-io/dls/tooltip/Tooltip";
import Spacing from "@galaxy-io/dls/containers/Spacing";
import { TooltipPosition } from "@galaxy-io/dls/tooltip/Tooltip";
import HelpIcon from "@galaxy-io/dls/icons/HelpIcon";

const EnumField = ({
  field,
  value,
  onChange,
  error,
  isDisabled = false,
  label,
}: FieldComponentProps) => {
  const [isEnumOpen, setIsEnumOpen] = useState(false);

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
      <Dropdown
        isOpen={isEnumOpen}
        onClose={() => setIsEnumOpen(false)}
        position={DropdownPosition.BOTTOM_START}
        fillWidth
        body={field.enum.map((enumValue) => (
          <DropdownItem
            key={enumValue}
            label={enumValue}
            onClick={() => {
              onChange(enumValue);
              setIsEnumOpen(false);
            }}
          />
        ))}
      >
        <DropdownButton
          label={(value as string) || `Select ${label}...`}
          isOpen={isEnumOpen}
          onClick={() => setIsEnumOpen(true)}
          isDisabled={isDisabled}
          fillWidth
        />
      </Dropdown>
      {error && (
        <Text size={TextSize.CAPTION} variant={TextVariant.ERROR}>
          {error}
        </Text>
      )}
      {field.help && !error && (
        <Text size={TextSize.CAPTION} variant={TextVariant.TERTIARY}>
          {field.help}
        </Text>
      )}
    </FlexWrapper>
  );
};

export default EnumField;
