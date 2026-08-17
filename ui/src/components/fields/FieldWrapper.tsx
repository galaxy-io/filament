import type { PropsWithChildren } from "react";

import Accordion, { AccordionVariant } from "@galaxy-io/dls/accordion/Accordion";
import FlexWrapper, { FlexDirection } from "@galaxy-io/dls/containers/FlexWrapper";
import { InputLabel } from "@galaxy-io/dls/inputs/Input";
import Text, { TextSize, TextVariant } from "@galaxy-io/dls/text/Text";

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
      <Accordion variant={AccordionVariant.PRIMARY} header={label} isOpenInitial>
        <FlexWrapper direction={FlexDirection.COLUMN} gap={4} fillWidth>
          {children}
          {errorText}
        </FlexWrapper>
      </Accordion>
    );
  }

  return (
    <FlexWrapper direction={FlexDirection.COLUMN} gap={4} fillWidth>
      <InputLabel label={label} labelTooltip={help || undefined} isRequired={isRequired} />
      {children}
      {errorText}
    </FlexWrapper>
  );
};

export default FieldWrapper;
