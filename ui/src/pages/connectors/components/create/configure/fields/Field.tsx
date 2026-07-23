import type { PropsWithChildren } from "react";

import { styled } from "@linaria/react";

import FlexWrapper, { AlignItems, FlexDirection } from "@galaxy-io/dls/containers/FlexWrapper";
import Spacing from "@galaxy-io/dls/containers/Spacing";
import HelpIcon from "@galaxy-io/dls/icons/HelpIcon";
import RequiredMarker from "@galaxy-io/dls/text/RequiredMarker";
import Text, { TextSize, TextVariant } from "@galaxy-io/dls/text/Text";
import { withTheme } from "@galaxy-io/dls/theme/GalaxyTheme";
import type { PropsWithTheme } from "@galaxy-io/dls/theme/types";
import Tooltip, { TooltipPosition } from "@galaxy-io/dls/tooltip/Tooltip";

interface FieldProps {
  label: string;
  help?: string;
  isRequired?: boolean;
  error?: string;
  isSection?: boolean;
}

const Section = withTheme(styled.div<PropsWithTheme>`
  width: 100%;
  overflow: hidden;
  border: 0.5px solid ${({ theme }) => theme.color.border.primary};
  border-radius: 8px;
  background-color: ${({ theme }) => theme.color.background.primary};
`);

const SectionHeader = withTheme(styled.div<PropsWithTheme>`
  padding: 10px 12px;
  border-bottom: 0.5px solid ${({ theme }) => theme.color.border.primary};
  background-color: ${({ theme }) => theme.color.background.base};
`);

const SectionBody = styled.div`
  padding: 12px;
`;

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
      <Section>
        <SectionHeader>
          <FieldLabel label={label} help={help} isRequired={isRequired} />
        </SectionHeader>
        <SectionBody>
          <FlexWrapper direction={FlexDirection.COLUMN} gap={6} fillWidth>
            {children}
            <FieldError error={error} />
          </FlexWrapper>
        </SectionBody>
      </Section>
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
