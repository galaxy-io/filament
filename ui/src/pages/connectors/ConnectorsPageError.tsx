import { WarningCircleIcon } from "@phosphor-icons/react";

import FlexWrapper, {
  AlignItems,
  FlexDirection,
  FlexGap,
  JustifyContent,
} from "@galaxy-io/dls/containers/FlexWrapper";
import Icon, { IconVariant } from "@galaxy-io/dls/icons/Icon";
import Text, { TextVariant } from "@galaxy-io/dls/text/Text";

interface ConnectorsPageErrorProps {
  message?: string;
}

const ConnectorsPageError = ({
  message = "Failed to load connectors. Please try again.",
}: ConnectorsPageErrorProps) => {
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

export default ConnectorsPageError;
