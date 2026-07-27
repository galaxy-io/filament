import { TrashIcon } from "@phosphor-icons/react";

import Button, { ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import FlexWrapper, {
  AlignItems,
  FlexDirection,
  FlexGap,
  JustifyContent,
} from "@galaxy-io/dls/containers/FlexWrapper";
import Text, { TextSize, TextVariant, TextWeight } from "@galaxy-io/dls/text/Text";
import Widget, { WidgetVariant } from "@galaxy-io/dls/widget/Widget";

export interface DangerZoneProps {
  title: string;
  description: string;
  onClick: () => void;
}

const DangerZone = ({ title, description, onClick }: DangerZoneProps) => {
  return (
    <Widget header="Danger zone" variant={WidgetVariant.ERROR} fillWidth noHover>
      <FlexWrapper
        justifyContent={JustifyContent.SPACE_BETWEEN}
        alignItems={AlignItems.CENTER}
        gap={12}
      >
        <FlexWrapper direction={FlexDirection.COLUMN} gap={FlexGap.XSMALL}>
          <Text weight={TextWeight.MEDIUM}>{title}</Text>
          <Text size={TextSize.BODY_SM} variant={TextVariant.SECONDARY}>
            {description}
          </Text>
        </FlexWrapper>
        <Button label="Delete" icon={TrashIcon} variant={ButtonVariant.ERROR} onClick={onClick} />
      </FlexWrapper>
    </Widget>
  );
};

export default DangerZone;
