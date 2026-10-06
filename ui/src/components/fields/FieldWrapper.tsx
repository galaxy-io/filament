import type { PropsWithChildren } from "react";

import Field from "@galaxy-io/dls/inputs/Field";
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
  if (isSection) {
    return (
      <Widget isCollapsible variant={WidgetVariant.PRIMARY} header={label} defaultIsOpen>
        <Flex alignItems={AlignItems.START} direction={FlexDirection.COLUMN} gap={4} fillWidth>
          {children}
          {error && (
            <Text size={TextSize.BODY_SM} variant={TextVariant.ERROR}>
              {error}
            </Text>
          )}
        </Flex>
      </Widget>
    );
  }

  return (
    <Field
      label={label}
      labelTooltip={help || undefined}
      isRequired={isRequired}
      error={error}
      fillWidth
    >
      {children}
    </Field>
  );
};

export default FieldWrapper;
