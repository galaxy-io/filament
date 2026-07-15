import type { ReactNode } from "react";

import type { Icon as PhosphorIcon } from "@phosphor-icons/react";

import FlexWrapper, {
  AlignItems,
  FlexDirection,
  FlexGap,
  JustifyContent,
} from "@galaxy-io/dls/containers/FlexWrapper";
import Icon, { IconVariant } from "@galaxy-io/dls/icons/Icon";
import Text, { TextSize, TextVariant } from "@galaxy-io/dls/text/Text";

import { IS_DEBUG } from "@/constants";

interface ErrorLayoutProps {
  icon: PhosphorIcon;
  header: string;
  message: string;
  error?: Error;
  actions?: ReactNode;
}

const ErrorLayout = ({ icon, header, message, error, actions }: ErrorLayoutProps) => {
  return (
    <FlexWrapper
      fillWidth
      fillHeight
      direction={FlexDirection.COLUMN}
      alignItems={AlignItems.CENTER}
      justifyContent={JustifyContent.CENTER}
      gap={FlexGap.MEDIUM}
    >
      <Icon component={icon} size={24} variant={IconVariant.SECONDARY} />
      <FlexWrapper
        direction={FlexDirection.COLUMN}
        alignItems={AlignItems.CENTER}
        gap={FlexGap.XSMALL}
      >
        <Text size={TextSize.HEADING_SM}>{header}</Text>
        <Text size={TextSize.BODY_MD} variant={TextVariant.SECONDARY}>
          {message}
        </Text>
        {IS_DEBUG && error && (
          <Text size={TextSize.BODY_SM} variant={TextVariant.TERTIARY} isMonospace isSelectable>
            {error.message}
          </Text>
        )}
      </FlexWrapper>
      {actions}
    </FlexWrapper>
  );
};

export default ErrorLayout;
