import { styled } from "@linaria/react";
import type { Icon as PhosphorIcon } from "@phosphor-icons/react";
import { XIcon } from "@phosphor-icons/react";

import Button, {
  ButtonSize,
  ButtonVariant,
} from "@galaxy-io/dls/buttons/Button";
import FlexItem from "@galaxy-io/dls/containers/FlexItem";
import FlexWrapper, { AlignItems } from "@galaxy-io/dls/containers/FlexWrapper";
import Icon, { IconVariant } from "@galaxy-io/dls/icons/Icon";
import Paragraph from "@galaxy-io/dls/text/Paragraph";
import Text, { TextVariant, TextWeight } from "@galaxy-io/dls/text/Text";

import {
  BASE_HEADER_SIZE_TO_GAP_MAP,
  BASE_HEADER_SIZE_TO_ICON_SIZE_MAP,
  BASE_HEADER_SIZE_TO_TITLE_SIZE_MAP,
} from "@/layouts/components/constants";
import { BaseHeaderSize } from "@/layouts/components/types";

interface BaseHeaderProps {
  title: string;
  icon?: PhosphorIcon;
  description?: string;
  actions?: React.ReactNode[];
  size?: BaseHeaderSize;
  onClose?: () => void;
}

const TitleWrapper = styled.div`
  display: flex;
  flex-direction: column;
  gap: 4px;
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
  const alignItems =
    onClose && description ? AlignItems.START : AlignItems.CENTER;
  return (
    <FlexWrapper alignItems={alignItems} gap={8} fillWidth>
      <TitleWrapper>
        <FlexWrapper
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
              isEllipsis
            >
              {title}
            </Text>
          </FlexItem>
        </FlexWrapper>
        {description && (
          <Paragraph variant={TextVariant.SECONDARY}>{description}</Paragraph>
        )}
      </TitleWrapper>
      <ActionsWrapper>
        {actions}
        {onClose && (
          <Button
            icon={XIcon}
            variant={ButtonVariant.TERTIARY}
            size={ButtonSize.SMALL}
            onClick={onClose}
          />
        )}
      </ActionsWrapper>
    </FlexWrapper>
  );
};

export default BaseHeader;
