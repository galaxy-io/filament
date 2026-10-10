import type { FC, PropsWithChildren } from "react";

import { css } from "@linaria/core";

import Flex, { FlexDirection } from "@galaxy-io/dls/layout/Flex";
import FlexItem, { FlexItemVariant } from "@galaxy-io/dls/layout/FlexItem";
import { Radius } from "@galaxy-io/dls/theme/enums";
import { t } from "@galaxy-io/dls/theme/tokens/t";

import {
  PIPELINE_SIDEBAR_ITEM_TO_PATH_MAP,
  PIPELINE_SIDEBAR_WIDTH,
} from "@/layouts/pipeline/constants";
import PipelineLayoutNavbar from "@/layouts/pipeline/PipelineLayoutNavbar";
import PipelineLayoutNavbarBackButton from "@/layouts/pipeline/PipelineLayoutNavbarBackButton";
import PipelineLayoutSidebar from "@/layouts/pipeline/PipelineLayoutSidebar";
import { PipelineSidebarItem } from "@/layouts/pipeline/types";

import { usePipelinePreviewVersion } from "@/pages/pipelines/hooks/usePipelinePreviewVersion";

import { useFilamentMatchRoute, useFilamentNavigate, usePipelineParams } from "@/module/hooks";
import { FilamentPath } from "@/module/paths";

const PREVIEW_ISLAND_CSS = css`
  border-color: ${t.color.border.error};
`;

const PipelineLayout: FC<PropsWithChildren> = ({ children }) => {
  const navigate = useFilamentNavigate();
  const { id } = usePipelineParams();

  const isPreview = usePipelinePreviewVersion() !== undefined;

  const matchRoute = useFilamentMatchRoute();
  const isHistoryActive = matchRoute(FilamentPath.PIPELINE_HISTORY, { params: { id } });
  const isSettingsActive = matchRoute(FilamentPath.PIPELINE_SETTINGS, { params: { id } });
  const getActiveItem = (): PipelineSidebarItem => {
    if (isHistoryActive) return PipelineSidebarItem.HISTORY;
    if (isSettingsActive) return PipelineSidebarItem.SETTINGS;
    return PipelineSidebarItem.CANVAS;
  };

  const handleItemClick = (item: PipelineSidebarItem) => {
    navigate({
      to: PIPELINE_SIDEBAR_ITEM_TO_PATH_MAP[item],
      params: { id },
    });
  };

  return (
    <Flex fillWidth height="100%">
      <Flex
        direction={FlexDirection.COLUMN}
        width={PIPELINE_SIDEBAR_WIDTH}
        height="100%"
        shrink={0}
      >
        <PipelineLayoutNavbarBackButton />
        <PipelineLayoutSidebar activeItem={getActiveItem()} onItemClick={handleItemClick} />
      </Flex>
      <Flex direction={FlexDirection.COLUMN} grow={1} basis={0} height="100%" minWidth={0}>
        <PipelineLayoutNavbar />
        <Flex grow={1} basis={0} fillWidth minHeight={0} padding={[0, 12, 12, 0]}>
          <FlexItem
            grow={1}
            basis={0}
            fillWidth
            minHeight={0}
            position="relative"
            variant={FlexItemVariant.PRIMARY}
            hasBorder
            radius={Radius.LG}
            overflow="hidden"
            className={isPreview ? PREVIEW_ISLAND_CSS : undefined}
          >
            {children}
          </FlexItem>
        </Flex>
      </Flex>
    </Flex>
  );
};

export default PipelineLayout;
