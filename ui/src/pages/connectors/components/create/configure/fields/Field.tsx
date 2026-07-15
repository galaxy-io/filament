import FlexWrapper, { AlignItems, FlexDirection } from "@galaxy-io/dls/containers/FlexWrapper";
import Spacing from "@galaxy-io/dls/containers/Spacing";
import HelpIcon from "@galaxy-io/dls/icons/HelpIcon";
import RequiredMarker from "@galaxy-io/dls/text/RequiredMarker";
import Text, { TextSize, TextVariant } from "@galaxy-io/dls/text/Text";
import Tooltip, { TooltipPosition } from "@galaxy-io/dls/tooltip/Tooltip";
import type { PropsWithChildren } from "react";

interface FieldProps {
  label: string;
  help?: string;
  isRequired?: boolean;
  error?: string;
}

const Field = ({ label, help, isRequired, error, children }: PropsWithChildren<FieldProps>) => (
  <FlexWrapper direction={FlexDirection.COLUMN} gap={6} fillWidth>
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
    {children}
    {error && (
      <Text size={TextSize.CAPTION} variant={TextVariant.ERROR}>
        {error}
      </Text>
    )}
  </FlexWrapper>
);

export default Field;
