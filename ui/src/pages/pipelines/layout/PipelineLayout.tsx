import type { PropsWithChildren } from "react";

import { styled } from "@linaria/react";
import { useNavigate } from "@tanstack/react-router";

import Chip, { ChipVariant } from "@galaxy-io/dls/chips/Chip";
import { withTheme } from "@galaxy-io/dls/theme/GalaxyTheme";
import type { PropsWithTheme } from "@galaxy-io/dls/theme/types";

import type { Pipeline, PipelineVersion } from "@/gen/ingestion/v1/pipelines_pb";

import { PIPELINE_SIDEBAR_WIDTH } from "@/pages/pipelines/layout/constants";
import PipelineLayoutBackButton from "@/pages/pipelines/layout/PipelineLayoutBackButton";
import PipelineLayoutNavbar from "@/pages/pipelines/layout/PipelineLayoutNavbar";
import PipelineLayoutSidebar from "@/pages/pipelines/layout/PipelineLayoutSidebar";
import { PipelineSidebarItem } from "@/pages/pipelines/layout/types";

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
      $isPreview ? theme.color.border.warning : theme.color.border.primary};
  border-radius: 6px;

  overflow: hidden;
`);

const PreviewChipOverlay = styled.div`
  position: absolute;
  top: 16px;
  right: 16px;
  z-index: 1002;
`;

interface PipelineLayoutProps {
  pipeline: Pipeline;
  currentVersion?: PipelineVersion;
  versions: PipelineVersion[];
  previewVersion: bigint | null;
  onPreviewVersionChange: (version: bigint | null) => void;
}

const PipelineLayout = ({
  pipeline,
  currentVersion,
  versions,
  previewVersion,
  onPreviewVersionChange,
  children,
}: PropsWithChildren<PipelineLayoutProps>) => {
  const navigate = useNavigate();

  const isPreview = previewVersion !== null;

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
      params: { id: pipeline.id },
    });
  };

  return (
    <LayoutWrapper>
      <LeftColumn>
        <PipelineLayoutBackButton />
        <PipelineLayoutSidebar activeItem={getActiveItem()} onItemClick={handleItemClick} />
      </LeftColumn>
      <RightColumn>
        <PipelineLayoutNavbar
          pipeline={pipeline}
          currentVersion={currentVersion}
          versions={versions}
          previewVersion={previewVersion}
          onPreviewVersionChange={onPreviewVersionChange}
        />
        <ContentWrapper>
          <ContentIsland $isPreview={isPreview}>
            {isPreview && (
              <PreviewChipOverlay>
                <Chip label={`Version ${previewVersion}`} variant={ChipVariant.WARNING} />
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
