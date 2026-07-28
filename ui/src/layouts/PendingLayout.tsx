import { CircleNotchIcon } from "@phosphor-icons/react";

import SpinAnimation from "@galaxy-io/dls/animations/SpinAnimation";
import FlexWrapper, {
  AlignItems,
  FlexDirection,
  FlexGap,
  JustifyContent,
} from "@galaxy-io/dls/containers/FlexWrapper";
import Icon, { IconVariant } from "@galaxy-io/dls/icons/Icon";
import Text, { TextSize, TextVariant } from "@galaxy-io/dls/text/Text";

interface PendingLayoutProps {
  message?: string;
}

const PendingLayout = ({ message }: PendingLayoutProps) => {
  return (
    <FlexWrapper
      fillWidth
      fillHeight
      direction={FlexDirection.COLUMN}
      alignItems={AlignItems.CENTER}
      justifyContent={JustifyContent.CENTER}
      gap={FlexGap.MEDIUM}
    >
      <SpinAnimation>
        <Icon component={CircleNotchIcon} size={24} variant={IconVariant.SECONDARY} />
      </SpinAnimation>
      {message && (
        <Text size={TextSize.BODY_MD} variant={TextVariant.SECONDARY}>
          {message}
        </Text>
      )}
    </FlexWrapper>
  );
};

export default PendingLayout;
