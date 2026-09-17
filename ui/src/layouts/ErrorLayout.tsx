import { type Icon as PhosphorIcon, WarningCircleIcon } from "@phosphor-icons/react";

import FlexWrapper, {
  AlignItems,
  FlexDirection,
  FlexGap,
  JustifyContent,
} from "@galaxy-io/dls/containers/FlexWrapper";
import Icon, { IconVariant } from "@galaxy-io/dls/icons/Icon";
import Text, { TextSize, TextVariant, TextWeight } from "@galaxy-io/dls/text/Text";

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
    <FlexWrapper
      fillWidth
      fillHeight
      direction={FlexDirection.COLUMN}
      alignItems={AlignItems.CENTER}
      justifyContent={JustifyContent.CENTER}
      gap={LAYOUT_SIZE_TO_GAP_MAP[size]}
    >
      <Icon
        component={icon}
        size={LAYOUT_SIZE_TO_GLYPH_SIZE_MAP[size]}
        variant={IconVariant.ERROR}
      />
      <FlexWrapper
        direction={FlexDirection.COLUMN}
        alignItems={AlignItems.CENTER}
        gap={FlexGap.SMALL}
      >
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
          <Text size={TextSize.CAPTION} variant={TextVariant.ERROR} isMonospace isSelectable>
            {error.message}
          </Text>
        )}
      </FlexWrapper>
      {actions}
    </FlexWrapper>
  );
};

export default ErrorLayout;
