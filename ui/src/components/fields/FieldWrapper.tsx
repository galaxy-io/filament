import type { PropsWithChildren } from "react";

// @dls-migrate field.InputLabel: Wrap the control in `<Field label>` instead of rendering a label above it.
import { InputLabel } from "@galaxy-io/dls/inputs/Input";
import Flex, { AlignItems, FlexDirection } from "@galaxy-io/dls/layout/Flex";
import Text, { TextSize, TextVariant } from "@galaxy-io/dls/text/Text";
import Widget, { WidgetVariant } from "@galaxy-io/dls/widget/Widget";

interface FieldWrapperProps {
  label: string;
  help?: string;
  isRequired?: boolean;
  error?: string;
  isSection?: boolean;
}

const FieldWrapper = ({
  label,
  help,
  isRequired,
  error,
  isSection = false,
  children,
}: PropsWithChildren<FieldWrapperProps>) => {
  const errorText = error && (
    <Text size={TextSize.BODY_SM} variant={TextVariant.ERROR}>
      {error}
    </Text>
  );

  if (isSection) {
    return (
      <Widget isCollapsible variant={WidgetVariant.PRIMARY} header={label} defaultIsOpen>
        <Flex alignItems={AlignItems.START} direction={FlexDirection.COLUMN} gap={4} fillWidth>
          {children}
          {errorText}
        </Flex>
      </Widget>
    );
  }

  return (
    <Flex alignItems={AlignItems.START} direction={FlexDirection.COLUMN} gap={4} fillWidth>
      <InputLabel label={label} labelTooltip={help || undefined} isRequired={isRequired} />
      {children}
      {errorText}
    </Flex>
  );
};

export default FieldWrapper;
