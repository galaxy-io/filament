import { type Icon as PhosphorIcon, WarningCircleIcon } from "@phosphor-icons/react";

import Icon, { IconVariant } from "@galaxy-io/dls/icons/Icon";
import Flex, { AlignItems, FlexDirection, JustifyContent } from "@galaxy-io/dls/layout/Flex";
import Text, { TextSize, TextVariant, TextWeight } from "@galaxy-io/dls/text/Text";
import { FontFamily } from "@galaxy-io/dls/theme/enums";

import {
  LAYOUT_SIZE_TO_GAP_MAP,
  LAYOUT_SIZE_TO_GLYPH_SIZE_MAP,
  LAYOUT_SIZE_TO_HEADER_SIZE_MAP,
  LAYOUT_SIZE_TO_MESSAGE_SIZE_MAP,
} from "@/layouts/constants";
import { LayoutSize } from "@/layouts/types";

import { IS_DEBUG } from "@/constants";

interface ErrorLayoutProps {
  size?: LayoutSize;
  icon?: PhosphorIcon;
  header?: string;
  message?: string;
  error?: Error;
  actions?: React.ReactNode;
}

const ErrorLayout = ({
  size = LayoutSize.MEDIUM,
  icon = WarningCircleIcon,
  header,
  message,
  error,
  actions,
}: ErrorLayoutProps) => {
  return (
    <Flex
      fillWidth
      height="100%"
      direction={FlexDirection.COLUMN}
      alignItems={AlignItems.CENTER}
      justifyContent={JustifyContent.CENTER}
      /* @dls-migrate layout.off-scale: Pick a value on the space scale (or a CSS-order tuple of them). */ gap={
        LAYOUT_SIZE_TO_GAP_MAP[size]
      }
    >
      <Icon
        component={icon}
        size={LAYOUT_SIZE_TO_GLYPH_SIZE_MAP[size]}
        variant={IconVariant.ERROR}
      />
      <Flex direction={FlexDirection.COLUMN} alignItems={AlignItems.CENTER} gap={8}>
        {header && (
          <Text size={LAYOUT_SIZE_TO_HEADER_SIZE_MAP[size]} weight={TextWeight.MEDIUM}>
            {header}
          </Text>
        )}
        {message && (
          <Text size={LAYOUT_SIZE_TO_MESSAGE_SIZE_MAP[size]} variant={TextVariant.SECONDARY}>
            {message}
          </Text>
        )}
        {IS_DEBUG && error && (
          <Text
            size={TextSize.CAPTION}
            variant={TextVariant.ERROR}
            family={FontFamily.MONO}
            isSelectable
          >
            {error.message}
          </Text>
        )}
      </Flex>
      {actions}
    </Flex>
  );
};

export default ErrorLayout;
