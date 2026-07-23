import type { PropsWithChildren } from "react";

import Accordion from "@galaxy-io/dls/accordion/Accordion";
import FlexWrapper, { AlignItems, FlexDirection } from "@galaxy-io/dls/containers/FlexWrapper";
import Spacing from "@galaxy-io/dls/containers/Spacing";
import HelpIcon from "@galaxy-io/dls/icons/HelpIcon";
import RequiredMarker from "@galaxy-io/dls/text/RequiredMarker";
import Text, { TextSize, TextVariant } from "@galaxy-io/dls/text/Text";
import Tooltip, { TooltipPosition } from "@galaxy-io/dls/tooltip/Tooltip";

interface FieldProps {
  label: string;
  help?: string;
  isRequired?: boolean;
  error?: string;
  isSection?: boolean;
}

const FieldLabel = ({
  label,
  help,
  isRequired,
}: Pick<FieldProps, "label" | "help" | "isRequired">) => (
  <FlexWrapper alignItems={AlignItems.CENTER} gap={1}>
    <Text variant={TextVariant.SECONDARY}>{label}</Text>
    {isRequired && <RequiredMarker />}
    {help && (
      <Spacing left={4}>
        <Tooltip body={help} position={TooltipPosition.RIGHT}>
          <HelpIcon />
        </Tooltip>
      </Spacing>
    )}
  </FlexWrapper>
);

const FieldError = ({ error }: Pick<FieldProps, "error">) =>
  error ? (
    <Text size={TextSize.CAPTION} variant={TextVariant.ERROR}>
      {error}
    </Text>
  ) : null;

const Field = ({
  label,
  help,
  isRequired,
  error,
  isSection = false,
  children,
}: PropsWithChildren<FieldProps>) => {
  if (isSection) {
    return (
      <Accordion
        header={label}
        isOpen={true}
        metric={
          (isRequired || help) && (
            <FlexWrapper alignItems={AlignItems.CENTER} gap={1}>
              {isRequired && <RequiredMarker />}
              {help && (
                <Tooltip body={help} position={TooltipPosition.RIGHT}>
                  <HelpIcon />
                </Tooltip>
              )}
            </FlexWrapper>
          )
        }
      >
        <FlexWrapper direction={FlexDirection.COLUMN} gap={6} fillWidth>
          {children}
          <FieldError error={error} />
        </FlexWrapper>
      </Accordion>
    );
  }

  return (
    <FlexWrapper direction={FlexDirection.COLUMN} gap={6} fillWidth>
      <FieldLabel label={label} help={help} isRequired={isRequired} />
      {children}
      <FieldError error={error} />
    </FlexWrapper>
  );
};

export default Field;
