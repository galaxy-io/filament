import FlexWrapper, {
  AlignItems,
  FlexDirection,
  JustifyContent,
} from "@galaxy-io/dls/containers/FlexWrapper";
import GalaxyLogomarkAnimation from "@galaxy-io/dls/icons/GalaxyLogomarkAnimation";
import Text, { TextVariant } from "@galaxy-io/dls/text/Text";

import {
  LAYOUT_SIZE_TO_GAP_MAP,
  LAYOUT_SIZE_TO_GLYPH_SIZE_MAP,
  LAYOUT_SIZE_TO_MESSAGE_SIZE_MAP,
} from "@/layouts/constants";
import { LayoutSize } from "@/layouts/types";

const PENDING_LAYOUT_ANIMATION_SPEED = 3;

interface PendingLayoutProps {
  size?: LayoutSize;
  message?: string;
}

const PendingLayout = ({ size = LayoutSize.MEDIUM, message }: PendingLayoutProps) => {
  return (
    <FlexWrapper
      fillWidth
      fillHeight
      direction={FlexDirection.COLUMN}
      alignItems={AlignItems.CENTER}
      justifyContent={JustifyContent.CENTER}
      gap={LAYOUT_SIZE_TO_GAP_MAP[size]}
    >
      <GalaxyLogomarkAnimation
        height={LAYOUT_SIZE_TO_GLYPH_SIZE_MAP[size]}
        speed={PENDING_LAYOUT_ANIMATION_SPEED}
      />
      {message && (
        <Text size={LAYOUT_SIZE_TO_MESSAGE_SIZE_MAP[size]} variant={TextVariant.SECONDARY}>
          {message}
        </Text>
      )}
    </FlexWrapper>
  );
};

export default PendingLayout;
