import FlexWrapper, {
  AlignItems,
  FlexDirection,
  FlexGap,
  JustifyContent,
} from "@galaxy-io/dls/containers/FlexWrapper";
import Text, { TextVariant, TextWeight } from "@galaxy-io/dls/text/Text";

import {
  LAYOUT_SIZE_TO_GAP_MAP,
  LAYOUT_SIZE_TO_HEADER_SIZE_MAP,
  LAYOUT_SIZE_TO_MESSAGE_SIZE_MAP,
} from "@/layouts/constants";
import { LayoutSize } from "@/layouts/types";

interface EmptyLayoutProps {
  size?: LayoutSize;
  icon?: React.ReactNode;
  header?: string;
  message?: string;
  actions?: React.ReactNode;
}

const EmptyLayout = ({
  size = LayoutSize.MEDIUM,
  icon,
  header,
  message,
  actions,
}: EmptyLayoutProps) => {
  return (
    <FlexWrapper
      fillWidth
      fillHeight
      direction={FlexDirection.COLUMN}
      alignItems={AlignItems.CENTER}
      justifyContent={JustifyContent.CENTER}
      gap={LAYOUT_SIZE_TO_GAP_MAP[size]}
    >
      {icon}
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
      </FlexWrapper>
      {actions}
    </FlexWrapper>
  );
};

export default EmptyLayout;
