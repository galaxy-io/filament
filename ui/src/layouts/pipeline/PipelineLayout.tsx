import type { PropsWithChildren } from "react";

import { css } from "@linaria/core";
import { styled } from "@linaria/react";
import { useNavigate, useParams } from "@tanstack/react-router";

import Chip, { ChipVariant } from "@galaxy-io/dls/chips/Chip";
import Flex, { FlexDirection } from "@galaxy-io/dls/layout/Flex";
import FlexItem, { FlexItemVariant } from "@galaxy-io/dls/layout/FlexItem";
import { Radius } from "@galaxy-io/dls/theme/enums";
import { t } from "@galaxy-io/dls/theme/tokens/t";

import {
  PIPELINE_PREVIEW_CHIP_Z_INDEX,
  PIPELINE_SIDEBAR_WIDTH,
} from "@/layouts/pipeline/constants";
import PipelineLayoutNavbar from "@/layouts/pipeline/PipelineLayoutNavbar";
import PipelineLayoutNavbarBackButton from "@/layouts/pipeline/PipelineLayoutNavbarBackButton";
import PipelineLayoutSidebar from "@/layouts/pipeline/PipelineLayoutSidebar";
import { PipelineSidebarItem } from "@/layouts/pipeline/types";

import { usePipelinePreviewVersion } from "@/pages/pipelines/hooks/usePipelinePreviewVersion";

import { useRouteMatch } from "@/hooks/useRouteMatch";

const PREVIEW_ISLAND_CSS = css`
  border-color: ${t.color.border.error};
`;

const PreviewChipOverlay = styled.div`
  position: absolute;
  top: 16px;
  left: 16px;
  z-index: ${PIPELINE_PREVIEW_CHIP_Z_INDEX};
`;

const PipelineLayout = ({ children }: PropsWithChildren) => {
  const navigate = useNavigate();
  const { id } = useParams({ from: "/_app/pipelines/$id" });

  const previewed = usePipelinePreviewVersion();
  const isPreview = previewed !== undefined;

  const { isRouteMatch: isHistoryActive } = useRouteMatch({
    route: "/pipelines/$id/history",
    fuzzy: false,
  });
  const { isRouteMatch: isSettingsActive } = useRouteMatch({
    route: "/pipelines/$id/settings",
    fuzzy: false,
  });
  const getActiveItem = (): PipelineSidebarItem => {
    if (isHistoryActive) return PipelineSidebarItem.HISTORY;
    if (isSettingsActive) return PipelineSidebarItem.SETTINGS;
    return PipelineSidebarItem.CANVAS;
  };

  const handleItemClick = (item: PipelineSidebarItem) => {
    navigate({
      to: `/pipelines/$id/${item}`,
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
            {isPreview && (
              <PreviewChipOverlay>
                <Chip label={`Version ${previewed.version}`} variant={ChipVariant.ERROR} />
              </PreviewChipOverlay>
            )}
            {children}
          </FlexItem>
        </Flex>
      </Flex>
    </Flex>
  );
};

export default PipelineLayout;
