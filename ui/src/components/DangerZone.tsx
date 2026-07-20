import { styled } from "@linaria/react";
import { TrashIcon } from "@phosphor-icons/react";

import Button, { ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import FlexWrapper, { FlexDirection, FlexGap } from "@galaxy-io/dls/containers/FlexWrapper";
import Text, { TextSize, TextVariant, TextWeight } from "@galaxy-io/dls/text/Text";
import Widget, { WidgetVariant } from "@galaxy-io/dls/widget/Widget";

export interface DangerZoneProps {
  title: string;
  description: string;
  buttonLabel: string;
  onAction: () => void;
}

const FieldRow = styled.div`
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
`;

const DangerZone = ({ title, description, buttonLabel, onAction }: DangerZoneProps) => {
  return (
    <Widget header="Danger zone" variant={WidgetVariant.ERROR} fillWidth noHover>
      <FieldRow>
        <FlexWrapper direction={FlexDirection.COLUMN} gap={FlexGap.XSMALL}>
          <Text weight={TextWeight.MEDIUM}>{title}</Text>
          <Text size={TextSize.BODY_SM} variant={TextVariant.SECONDARY}>
            {description}
          </Text>
        </FlexWrapper>
        <Button
          label={buttonLabel}
          icon={TrashIcon}
          variant={ButtonVariant.ERROR}
          onClick={onAction}
        />
      </FieldRow>
    </Widget>
  );
};

export default DangerZone;
