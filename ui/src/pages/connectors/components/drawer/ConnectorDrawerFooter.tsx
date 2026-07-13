import { TrashIcon } from "@phosphor-icons/react";
import { styled } from "@linaria/react";

import Button, { ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import FlexWrapper, { FlexDirection, FlexGap } from "@galaxy-io/dls/containers/FlexWrapper";
import Text, { TextSize, TextVariant, TextWeight } from "@galaxy-io/dls/text/Text";
import Widget, { WidgetVariant } from "@galaxy-io/dls/widget/Widget";

const FieldRow = styled.div`
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
`;

interface ConnectorDrawerFooterProps {
  onDelete: () => void;
}

const ConnectorDrawerFooter = ({ onDelete }: ConnectorDrawerFooterProps) => {
  return (
    <Widget header="Danger zone" variant={WidgetVariant.ERROR} fillWidth noHover>
      <FieldRow>
        <FlexWrapper direction={FlexDirection.COLUMN} gap={FlexGap.XSMALL}>
          <Text weight={TextWeight.MEDIUM}>Delete connector</Text>
          <Text size={TextSize.BODY_SM} variant={TextVariant.SECONDARY}>
            This will permanently delete this connector and all of its data.
          </Text>
        </FlexWrapper>
        <Button
          label="Delete connector"
          icon={TrashIcon}
          variant={ButtonVariant.ERROR}
          onClick={onDelete}
        />
      </FieldRow>
    </Widget>
  );
};

export default ConnectorDrawerFooter;
