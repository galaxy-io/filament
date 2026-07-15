import FlexWrapper, {
  AlignItems,
  FlexDirection,
} from "@galaxy-io/dls/containers/FlexWrapper";
import CodeEditor from "@galaxy-io/dls/editor/CodeEditor";
import RequiredMarker from "@galaxy-io/dls/text/RequiredMarker";
import Text, { TextSize, TextVariant } from "@galaxy-io/dls/text/Text";

import type { FieldComponentProps } from "./types";
import HelpIcon from "@galaxy-io/dls/icons/HelpIcon";
import Spacing from "@galaxy-io/dls/containers/Spacing";
import Tooltip, { TooltipPosition } from "@galaxy-io/dls/tooltip/Tooltip";

const ObjectField = ({
  field,
  value,
  onChange,
  error,
  isDisabled = false,
  label,
}: FieldComponentProps) => {
  const displayValue =
    typeof value === "string"
      ? value
      : value
        ? JSON.stringify(value, null, 2)
        : "";

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
      <CodeEditor
        content={displayValue}
        onChange={(v) => onChange(v)}
        placeholder={field.help || "Enter JSON..."}
        lang="json"
        isReadOnly={isDisabled}
      />
      {error && (
        <Text size={TextSize.CAPTION} variant={TextVariant.ERROR}>
          {error}
        </Text>
      )}
    </FlexWrapper>
  );
};

export default ObjectField;
