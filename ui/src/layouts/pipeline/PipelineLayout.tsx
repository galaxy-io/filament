import type { PropsWithChildren } from "react";

import { styled } from "@linaria/react";
import { useNavigate, useParams } from "@tanstack/react-router";

import Chip, { ChipVariant } from "@galaxy-io/dls/chips/Chip";
import { withTheme } from "@galaxy-io/dls/theme/GalaxyTheme";
import type { PropsWithTheme } from "@galaxy-io/dls/theme/types";

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

const LayoutWrapper = withTheme(styled.div<PropsWithTheme>`
  width: 100%;
  height: 100%;

  display: flex;

  background-color: ${({ theme }) => theme.color.background.base};
`);

const LeftColumn = withTheme(styled.div<PropsWithTheme>`
  width: ${PIPELINE_SIDEBAR_WIDTH}px;
  height: 100%;

  flex-shrink: 0;

  display: flex;
  flex-direction: column;

  background-color: ${({ theme }) => theme.color.background.base};
`);

const RightColumn = styled.div`
  flex: 1;
  height: 100%;
  min-width: 0;

  display: flex;
  flex-direction: column;
`;

const ContentWrapper = withTheme(styled.div<PropsWithTheme>`
  flex: 1;
  width: 100%;
  min-height: 0;

  padding: 0 12px 12px 0;

  display: flex;

  background-color: ${({ theme }) => theme.color.background.base};
`);

const ContentIsland = withTheme(styled.div<PropsWithTheme<{ $isPreview?: boolean }>>`
  position: relative;

  flex: 1;
  width: 100%;
  min-height: 0;

  background-color: ${({ theme }) => theme.color.background.primary};

  border: 0.5px solid
    ${({ $isPreview, theme }) =>
      $isPreview ? theme.color.border.error : theme.color.border.primary};
  border-radius: 6px;

  overflow: hidden;
`);

const PreviewChipOverlay = styled.div`
  position: absolute;
  top: 16px;
  left: 16px;
  z-index: ${PIPELINE_PREVIEW_CHIP_Z_INDEX};
`;

const PipelineLayout = ({ children }: PropsWithChildren) => {
  const navigate = useNavigate();
  const { id } = useParams({ from: "/pipelines/$id" });

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
    <LayoutWrapper>
      <LeftColumn>
        <PipelineLayoutNavbarBackButton />
        <PipelineLayoutSidebar activeItem={getActiveItem()} onItemClick={handleItemClick} />
      </LeftColumn>
      <RightColumn>
        <PipelineLayoutNavbar />
        <ContentWrapper>
          <ContentIsland $isPreview={isPreview}>
            {isPreview && (
              <PreviewChipOverlay>
                <Chip label={`Version ${previewed.version}`} variant={ChipVariant.ERROR} />
              </PreviewChipOverlay>
            )}
            {children}
          </ContentIsland>
        </ContentWrapper>
      </RightColumn>
    </LayoutWrapper>
  );
};

export default PipelineLayout;
