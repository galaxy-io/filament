import { type FC, type PropsWithChildren, type ReactNode, useState } from "react";

import { styled } from "@linaria/react";
import { CaretDownIcon, CaretUpIcon, type Icon as PhosphorIcon } from "@phosphor-icons/react";

import Icon, { IconVariant } from "@galaxy-io/dls/icons/Icon";
import Divider from "@galaxy-io/dls/layout/Divider";
import Flex, { AlignItems, FlexDirection } from "@galaxy-io/dls/layout/Flex";
import FlexItem from "@galaxy-io/dls/layout/FlexItem";
import { FOCUS_RING, HAIRLINE_WIDTH, INTERACTIVE_RESET } from "@galaxy-io/dls/styles/mixins";
import Text, { TextSize, TextWeight } from "@galaxy-io/dls/text/Text";
import { t } from "@galaxy-io/dls/theme/tokens/t";

const CreatePipelineModalResourcesPaneSectionIsland = styled.div`
  display: flex;
  flex-direction: column;
  overflow: hidden;
  border: ${HAIRLINE_WIDTH} solid ${t.color.border.primary};
  border-radius: ${t.radius.lg};
  background-color: ${t.color.background.secondary};
`;

const CreatePipelineModalResourcesPaneSectionBody = styled.div<{ $isOpen: boolean }>`
  display: grid;
  grid-template-rows: ${({ $isOpen }) => ($isOpen ? "1fr" : "0fr")};
  visibility: ${({ $isOpen }) => ($isOpen ? "visible" : "hidden")};
  transition:
    grid-template-rows ${t.duration.base} ease-in-out,
    visibility 0s linear ${({ $isOpen }) => ($isOpen ? "0s" : t.duration.base)};

  @media (prefers-reduced-motion: reduce) {
    transition: none;
  }
`;

const CreatePipelineModalResourcesPaneSectionToggle = styled.button`
  ${INTERACTIVE_RESET}
  ${FOCUS_RING}
  width: 100%;
  min-height: 40px;
  flex-shrink: 0;
  padding: 0 ${t.space[12]};
  cursor: pointer;
  transition: background-color ${t.duration.fast};

  &:hover {
    background-color: ${t.color.background.hovered};
  }
`;

interface CreatePipelineModalResourcesPaneSectionProps {
  icon: PhosphorIcon;
  header: string;
  trailing: ReactNode;
}

const CreatePipelineModalResourcesPaneSection: FC<
  PropsWithChildren<CreatePipelineModalResourcesPaneSectionProps>
> = ({ icon, header, trailing, children }) => {
  const [isOpen, setIsOpen] = useState(true);

  return (
    <CreatePipelineModalResourcesPaneSectionIsland>
      <CreatePipelineModalResourcesPaneSectionToggle
        type="button"
        aria-expanded={isOpen}
        onClick={() => setIsOpen(!isOpen)}
      >
        <Flex alignItems={AlignItems.CENTER} gap={8}>
          <Icon component={icon} size={15} variant={IconVariant.SECONDARY} />
          <FlexItem grow={1}>
            <Text size={TextSize.BODY_MD} weight={TextWeight.MEDIUM}>
              {header}
            </Text>
          </FlexItem>
          {trailing}
          <Icon
            component={isOpen ? CaretUpIcon : CaretDownIcon}
            size={12}
            variant={IconVariant.SECONDARY}
          />
        </Flex>
      </CreatePipelineModalResourcesPaneSectionToggle>
      <CreatePipelineModalResourcesPaneSectionBody $isOpen={isOpen}>
        <Flex direction={FlexDirection.COLUMN} minHeight={0} overflow="hidden">
          <Divider />
          {children}
        </Flex>
      </CreatePipelineModalResourcesPaneSectionBody>
    </CreatePipelineModalResourcesPaneSectionIsland>
  );
};

export default CreatePipelineModalResourcesPaneSection;
