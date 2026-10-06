import { styled } from "@linaria/react";

import Icon, { IconVariant, IconWeight } from "@galaxy-io/dls/icons/Icon";
import Flex, { AlignItems, FlexDirection } from "@galaxy-io/dls/layout/Flex";
import { t } from "@galaxy-io/dls/theme/tokens/t";

import {
  PIPELINE_SIDEBAR_BUTTON_SIZE,
  PIPELINE_SIDEBAR_ITEM_TO_ICON_MAP,
  PIPELINE_SIDEBAR_ITEM_TO_LABEL_MAP,
  PIPELINE_SIDEBAR_ITEMS,
  PIPELINE_SIDEBAR_WIDTH,
} from "@/layouts/pipeline/constants";
import type { PipelineSidebarItem } from "@/layouts/pipeline/types";

const SidebarWrapper = styled.div`
  width: ${PIPELINE_SIDEBAR_WIDTH}px;
  flex: 1;

  display: flex;
  flex-direction: column;
  align-items: center;

  padding: 12px 0;

  background-color: ${t.color.background.base};
`;

const SidebarButton = styled.button<{ $isActive?: boolean }>`
  width: ${PIPELINE_SIDEBAR_BUTTON_SIZE}px;
  height: ${PIPELINE_SIDEBAR_BUTTON_SIZE}px;

  display: flex;
  align-items: center;
  justify-content: center;

  background-color: ${({ $isActive }) => ($isActive ? t.color.background.tertiary : "transparent")};
  border: none;
  border-radius: 6px;

  color: ${({ $isActive }) => ($isActive ? t.color.text.primary : t.color.text.secondary)};

  cursor: pointer;

  transition:
    background-color 100ms ease,
    color 100ms ease;

  &:hover {
    background-color: ${t.color.background.tertiary};
    color: ${t.color.text.primary};
  }
`;

interface PipelineLayoutSidebarProps {
  activeItem: PipelineSidebarItem;
  onItemClick: (item: PipelineSidebarItem) => void;
}

const PipelineLayoutSidebar = ({ activeItem, onItemClick }: PipelineLayoutSidebarProps) => {
  return (
    <SidebarWrapper>
      <Flex alignItems={AlignItems.START} direction={FlexDirection.COLUMN} gap={12}>
        {PIPELINE_SIDEBAR_ITEMS.map((item) => {
          const isActive = activeItem === item;
          return (
            <SidebarButton
              key={item}
              $isActive={isActive}
              onClick={() => onItemClick(item)}
              aria-label={PIPELINE_SIDEBAR_ITEM_TO_LABEL_MAP[item]}
              aria-current={isActive ? "page" : undefined}
              title={PIPELINE_SIDEBAR_ITEM_TO_LABEL_MAP[item]}
            >
              <Icon
                component={PIPELINE_SIDEBAR_ITEM_TO_ICON_MAP[item]}
                size={18}
                weight={isActive ? IconWeight.FILL : IconWeight.REGULAR}
                variant={isActive ? undefined : IconVariant.TERTIARY}
              />
            </SidebarButton>
          );
        })}
      </Flex>
    </SidebarWrapper>
  );
};

export default PipelineLayoutSidebar;
