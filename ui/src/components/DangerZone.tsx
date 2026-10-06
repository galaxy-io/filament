import { TrashIcon } from "@phosphor-icons/react";

import Button, { ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import Flex, { AlignItems, FlexDirection, JustifyContent } from "@galaxy-io/dls/layout/Flex";
import Text, { TextSize, TextVariant, TextWeight } from "@galaxy-io/dls/text/Text";
import Widget, { WidgetVariant } from "@galaxy-io/dls/widget/Widget";

export interface DangerZoneProps {
  title: string;
  description: string;
  onDelete: () => void;
  isDisabled?: boolean;
}

const DangerZone = ({ title, description, onDelete, isDisabled }: DangerZoneProps) => {
  return (
    <Widget /* @dls-migrate widget.header: `header` now renders a header row: check that it should show. */
      header="Danger zone" /* @dls-migrate widget.WidgetVariant.ERROR: Removed: use a neutral card with an `Alert`, a labeled `Beacon` or a `Chip` inside. */
      variant={
        WidgetVariant.ERROR
      } /* @dls-migrate widget.fillWidth: Grow the card with a `FlexItem` or a `Grid` track. */
      fillWidth
    >
      <Flex justifyContent={JustifyContent.SPACE_BETWEEN} alignItems={AlignItems.CENTER} gap={12}>
        <Flex alignItems={AlignItems.START} direction={FlexDirection.COLUMN} gap={4}>
          <Text weight={TextWeight.MEDIUM}>{title}</Text>
          <Text size={TextSize.BODY_SM} variant={TextVariant.SECONDARY}>
            {description}
          </Text>
        </Flex>
        <Button
          label="Delete"
          icon={TrashIcon}
          variant={ButtonVariant.ERROR}
          onClick={onDelete}
          isDisabled={isDisabled}
        />
      </Flex>
    </Widget>
  );
};

export default DangerZone;
