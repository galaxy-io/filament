import { styled } from "@linaria/react";
import { XIcon } from "@phosphor-icons/react";

import Button, { ButtonSize, ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import FlexWrapper, { AlignItems, JustifyContent } from "@galaxy-io/dls/containers/FlexWrapper";
import Text, { TextVariant, TextWeight } from "@galaxy-io/dls/text/Text";

import { BASE_HEADER_SIZE_TO_TITLE_SIZE_MAP } from "@/layouts/components/constants";
import { BaseHeaderSize } from "@/layouts/components/types";

interface BaseHeaderProps {
  title: string | React.ReactNode;
  description?: string | React.ReactNode;
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
  description,
  actions,
  size = BaseHeaderSize.MEDIUM,
  onClose,
}: BaseHeaderProps) => {
  const alignItems = onClose && description ? AlignItems.START : AlignItems.CENTER;
  return (
    <FlexWrapper alignItems={alignItems} justifyContent={JustifyContent.SPACE_BETWEEN} fillWidth>
      <TitleWrapper>
        {typeof title === "string" ? (
          <Text
            size={BASE_HEADER_SIZE_TO_TITLE_SIZE_MAP[size]}
            weight={TextWeight.MEDIUM}
            isEllipsis
          >
            {title}
          </Text>
        ) : (
          title
        )}
        {description &&
          (typeof description === "string" ? (
            <Text variant={TextVariant.TERTIARY}>{description}</Text>
          ) : (
            description
          ))}
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
