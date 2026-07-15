import { styled } from "@linaria/react";

import FlexWrapper, { FlexDirection, FlexGap } from "@galaxy-io/dls/containers/FlexWrapper";
import Icon, { IconVariant, IconWeight } from "@galaxy-io/dls/icons/Icon";
import { withTheme } from "@galaxy-io/dls/theme/GalaxyTheme";
import type { PropsWithTheme } from "@galaxy-io/dls/theme/types";

import {
  PIPELINE_SIDEBAR_BUTTON_SIZE,
  PIPELINE_SIDEBAR_ITEM_TO_ICON_MAP,
  PIPELINE_SIDEBAR_ITEMS,
  PIPELINE_SIDEBAR_WIDTH,
} from "@/layouts/pipeline/constants";
import type { PipelineSidebarItem } from "@/layouts/pipeline/types";

const SidebarWrapper = withTheme(styled.div<PropsWithTheme>`
  width: ${PIPELINE_SIDEBAR_WIDTH}px;
  flex: 1;

  display: flex;
  flex-direction: column;
  align-items: center;

  padding: 12px 0;

  background-color: ${({ theme }) => theme.color.background.base};
`);

const SidebarButton = withTheme(styled.button<PropsWithTheme<{ $isActive?: boolean }>>`
  width: ${PIPELINE_SIDEBAR_BUTTON_SIZE}px;
  height: ${PIPELINE_SIDEBAR_BUTTON_SIZE}px;

  display: flex;
  align-items: center;
  justify-content: center;

  background-color: ${({ theme, $isActive }) =>
    $isActive ? theme.color.background.tertiary : "transparent"};
  border: none;
  border-radius: 6px;

  color: ${({ theme, $isActive }) =>
    $isActive ? theme.color.text.primary : theme.color.text.secondary};

  cursor: pointer;

  transition:
    background-color 100ms ease,
    color 100ms ease;

  &:hover {
    background-color: ${({ theme }) => theme.color.background.tertiary};
    color: ${({ theme }) => theme.color.text.primary};
  }
`);

interface PipelineLayoutSidebarProps {
  activeItem: PipelineSidebarItem;
  onItemClick: (item: PipelineSidebarItem) => void;
}

const PipelineLayoutSidebar = ({ activeItem, onItemClick }: PipelineLayoutSidebarProps) => {
  return (
    <SidebarWrapper>
      <FlexWrapper direction={FlexDirection.COLUMN} gap={FlexGap.MEDIUM}>
        {PIPELINE_SIDEBAR_ITEMS.map((item) => {
          const isActive = activeItem === item;
          return (
            <SidebarButton key={item} $isActive={isActive} onClick={() => onItemClick(item)}>
              <Icon
                component={PIPELINE_SIDEBAR_ITEM_TO_ICON_MAP[item]}
                size={18}
                weight={isActive ? IconWeight.FILL : IconWeight.REGULAR}
                variant={isActive ? undefined : IconVariant.TERTIARY}
              />
            </SidebarButton>
          );
        })}
      </FlexWrapper>
    </SidebarWrapper>
  );
};

export default PipelineLayoutSidebar;
