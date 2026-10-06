import Flex, { AlignItems, FlexDirection, JustifyContent } from "@galaxy-io/dls/layout/Flex";
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
    <Flex
      fillWidth
      height="100%"
      direction={FlexDirection.COLUMN}
      alignItems={AlignItems.CENTER}
      justifyContent={JustifyContent.CENTER}
      gap={LAYOUT_SIZE_TO_GAP_MAP[size]}
    >
      {icon}
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
      </Flex>
      {actions}
    </Flex>
  );
};

export default EmptyLayout;
