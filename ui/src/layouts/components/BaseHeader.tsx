import { styled } from "@linaria/react";
import { XIcon } from "@phosphor-icons/react";

import Button, { ButtonSize, ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import Text, { TextVariant, TextWeight } from "@galaxy-io/dls/text/Text";

import { BASE_HEADER_SIZE_TO_TITLE_SIZE_MAP } from "@/layouts/components/constants";
import { BaseHeaderSize } from "@/layouts/components/types";

interface BaseHeaderProps {
  title: string;
  description?: string;
  actions?: React.ReactNode[];
  size?: BaseHeaderSize;
  onClose?: () => void;
}

const Wrapper = styled.div<{ $alignItems: string }>`
  display: flex;
  align-items: ${({ $alignItems }) => $alignItems};
  gap: 8px;
  width: 100%;
`;

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
  const alignItems = onClose && description ? "flex-start" : "center";
  return (
    <Wrapper $alignItems={alignItems}>
      <TitleWrapper>
        <Text size={BASE_HEADER_SIZE_TO_TITLE_SIZE_MAP[size]} weight={TextWeight.MEDIUM} isEllipsis>
          {title}
        </Text>
        {description && <Text variant={TextVariant.SECONDARY}>{description}</Text>}
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
    </Wrapper>
  );
};

export default BaseHeader;
