import { WarningCircleIcon } from "@phosphor-icons/react";

import FlexWrapper, {
  AlignItems,
  FlexDirection,
  FlexGap,
  JustifyContent,
} from "@galaxy-io/dls/containers/FlexWrapper";
import Icon, { IconVariant } from "@galaxy-io/dls/icons/Icon";
import Text, { TextVariant } from "@galaxy-io/dls/text/Text";

interface PipelinesPageErrorProps {
  message?: string;
}

const PipelinesPageError = ({
  message = "Failed to load pipelines. Please try again.",
}: PipelinesPageErrorProps) => {
  return (
    <FlexWrapper
      fillWidth
      fillHeight
      direction={FlexDirection.COLUMN}
      alignItems={AlignItems.CENTER}
      justifyContent={JustifyContent.CENTER}
      gap={FlexGap.MEDIUM}
    >
      <Icon
        component={WarningCircleIcon}
        size={20}
        variant={IconVariant.ERROR}
      />
      <Text variant={TextVariant.ERROR}>{message}</Text>
    </FlexWrapper>
  );
};

export default PipelinesPageError;
