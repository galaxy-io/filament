import type { FC, ReactNode } from "react";

import { type Icon as PhosphorIcon, WarningCircleIcon } from "@phosphor-icons/react";

import EmptyState, { EmptyStateVariant } from "@galaxy-io/dls/feedback/EmptyState";
import Flex, { AlignItems, FlexDirection, JustifyContent } from "@galaxy-io/dls/layout/Flex";
import Text, { TextSize, TextVariant } from "@galaxy-io/dls/text/Text";
import { FontFamily } from "@galaxy-io/dls/theme/enums";

import { LAYOUT_SIZE_TO_EMPTY_STATE_SIZE_MAP } from "@/layouts/constants";
import { LayoutSize } from "@/layouts/types";

import { IS_DEBUG } from "@/constants";

interface ErrorLayoutProps {
  size?: LayoutSize;
  icon?: PhosphorIcon;
  header: string;
  description?: string;
  error?: Error;
  actions?: ReactNode;
}

const ErrorLayout: FC<ErrorLayoutProps> = ({
  size = LayoutSize.MEDIUM,
  icon = WarningCircleIcon,
  header,
  description,
  error,
  actions,
}) => {
  return (
    <Flex
      fillWidth
      height="100%"
      direction={FlexDirection.COLUMN}
      alignItems={AlignItems.CENTER}
      justifyContent={JustifyContent.CENTER}
    >
      <EmptyState
        size={LAYOUT_SIZE_TO_EMPTY_STATE_SIZE_MAP[size]}
        variant={EmptyStateVariant.ERROR}
        icon={icon}
        header={header}
        description={description}
        actions={actions}
      />
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
  );
};

export default ErrorLayout;
