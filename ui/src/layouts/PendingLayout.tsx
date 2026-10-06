import GalaxyLogomarkAnimation from "@galaxy-io/dls/brand/GalaxyLogomarkAnimation";
import Flex, { AlignItems, FlexDirection, JustifyContent } from "@galaxy-io/dls/layout/Flex";
import Text, { TextVariant } from "@galaxy-io/dls/text/Text";

import { LAYOUT_SIZE_TO_GAP_MAP, LAYOUT_SIZE_TO_MESSAGE_SIZE_MAP } from "@/layouts/constants";
import { LayoutSize } from "@/layouts/types";

const PENDING_LAYOUT_ANIMATION_SPEED = 4;

const PENDING_LAYOUT_SIZE_TO_MARK_SIZE_MAP: Record<LayoutSize, number> = {
  [LayoutSize.SMALL]: 16,
  [LayoutSize.MEDIUM]: 20,
  [LayoutSize.LARGE]: 28,
};

interface PendingLayoutProps {
  size?: LayoutSize;
  message?: string;
}

const PendingLayout = ({ size = LayoutSize.MEDIUM, message }: PendingLayoutProps) => {
  return (
    <Flex
      fillWidth
      height="100%"
      direction={FlexDirection.COLUMN}
      alignItems={AlignItems.CENTER}
      justifyContent={JustifyContent.CENTER}
      gap={LAYOUT_SIZE_TO_GAP_MAP[size]}
    >
      <GalaxyLogomarkAnimation
        size={PENDING_LAYOUT_SIZE_TO_MARK_SIZE_MAP[size]}
        speed={PENDING_LAYOUT_ANIMATION_SPEED}
      />
      {message && (
        <Text size={LAYOUT_SIZE_TO_MESSAGE_SIZE_MAP[size]} variant={TextVariant.SECONDARY}>
          {message}
        </Text>
      )}
    </Flex>
  );
};

export default PendingLayout;
