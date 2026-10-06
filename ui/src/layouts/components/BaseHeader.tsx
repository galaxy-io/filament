import { styled } from "@linaria/react";
import type { Icon as PhosphorIcon } from "@phosphor-icons/react";
import { XIcon } from "@phosphor-icons/react";

import Button, { ButtonSize, ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import Icon, { IconVariant } from "@galaxy-io/dls/icons/Icon";
import Flex, { AlignItems } from "@galaxy-io/dls/layout/Flex";
import FlexItem from "@galaxy-io/dls/layout/FlexItem";
import Text, { TextSize, TextVariant, TextWeight } from "@galaxy-io/dls/text/Text";
import type { Space } from "@galaxy-io/dls/theme/enums";

export enum BaseHeaderSize {
  SMALL = "SMALL",
  MEDIUM = "MEDIUM",
  LARGE = "LARGE",
}

interface BaseHeaderProps {
  title: string;
  size?: BaseHeaderSize;
  icon?: PhosphorIcon;
  description?: string;
  actions?: React.ReactNode[];
  onClose?: () => void;
}

const BASE_HEADER_SIZE_TO_TITLE_SIZE_MAP: Record<BaseHeaderSize, TextSize> = {
  [BaseHeaderSize.SMALL]: TextSize.BODY_MD,
  [BaseHeaderSize.MEDIUM]: TextSize.BODY_LG,
  [BaseHeaderSize.LARGE]: TextSize.HEADING_SM,
};

const BASE_HEADER_SIZE_TO_ICON_SIZE_MAP: Record<BaseHeaderSize, number> = {
  [BaseHeaderSize.SMALL]: 14,
  [BaseHeaderSize.MEDIUM]: 16,
  [BaseHeaderSize.LARGE]: 20,
};

const BASE_HEADER_SIZE_TO_GAP_MAP: Record<BaseHeaderSize, Space> = {
  [BaseHeaderSize.SMALL]: 8,
  [BaseHeaderSize.MEDIUM]: 12,
  [BaseHeaderSize.LARGE]: 16,
};

const TitleWrapper = styled.div`
  display: flex;
  flex-direction: column;
  gap: 2px;
  flex: 1;
  min-width: 0;
  overflow: hidden;
`;

const ActionsWrapper = styled.div`
  display: flex;
  align-items: center;
  gap: 8px;
  flex-shrink: 0;
`;

const BaseHeader = ({
  title,
  icon,
  description,
  actions,
  size = BaseHeaderSize.MEDIUM,
  onClose,
}: BaseHeaderProps) => {
  const alignItems = onClose && description ? AlignItems.START : AlignItems.CENTER;
  return (
    <Flex alignItems={alignItems} gap={8} fillWidth>
      <TitleWrapper>
        <Flex
          alignItems={AlignItems.CENTER}
          gap={BASE_HEADER_SIZE_TO_GAP_MAP[size]}
          minWidth={0}
          fillWidth
        >
          {icon && (
            <FlexItem shrink={0}>
              <Icon
                component={icon}
                size={BASE_HEADER_SIZE_TO_ICON_SIZE_MAP[size]}
                variant={IconVariant.SECONDARY}
              />
            </FlexItem>
          )}
          <FlexItem grow={1} minWidth={0}>
            <Text
              size={BASE_HEADER_SIZE_TO_TITLE_SIZE_MAP[size]}
              weight={TextWeight.MEDIUM}
              lineClamp={1}
            >
              {title}
            </Text>
          </FlexItem>
        </Flex>
        {description && (
          <Text isProse variant={TextVariant.SECONDARY}>
            {description}
          </Text>
        )}
      </TitleWrapper>
      <ActionsWrapper>
        {actions}
        {onClose && (
          <Button
            icon={XIcon}
            ariaLabel="Close"
            variant={ButtonVariant.SECONDARY}
            size={ButtonSize.SMALL}
            onClick={onClose}
          />
        )}
      </ActionsWrapper>
    </Flex>
  );
};

export default BaseHeader;
