import type { PropsWithChildren } from "react";
import { useState } from "react";

import { styled } from "@linaria/react";
import { useNavigate } from "@tanstack/react-router";

import { withTheme } from "@galaxy-io/dls/theme/GalaxyTheme";
import type { PropsWithTheme } from "@galaxy-io/dls/theme/types";

import { useRouteMatch } from "@/hooks/useRouteMatch";
import PipelineLayoutNavbar from "@/layouts/pipeline/PipelineLayoutNavbar";
import PipelineLayoutSidebar from "@/layouts/pipeline/PipelineLayoutSidebar";
import { PIPELINE_SIDEBAR_WIDTH } from "@/layouts/pipeline/constants";
import { PipelineSidebarItem, PipelineStatus } from "@/layouts/pipeline/types";

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

const ContentIsland = withTheme(styled.div<PropsWithTheme>`
  flex: 1;
  width: 100%;
  min-height: 0;

  background-color: ${({ theme }) => theme.color.background.primary};

  border: 0.5px solid ${({ theme }) => theme.color.border.primary};
  border-radius: 6px;

  overflow: hidden;
`);

interface PipelineLayoutProps extends PropsWithChildren {
  pipelineId: string;
  name: string;
  status: PipelineStatus;
  source: string;
  sinks: string[];
}

const PipelineLayout = ({
  pipelineId,
  name,
  status,
  source,
  sinks,
  children,
}: PipelineLayoutProps) => {
  const navigate = useNavigate();
  const [isEnabled, setIsEnabled] = useState(status === PipelineStatus.ACTIVE);

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
      params: { id: pipelineId },
    });
  };

  const handleRun = () => {
    // TODO: Implement run logic
  };

  return (
    <LayoutWrapper>
      <LeftColumn>
        <PipelineLayoutNavbar.BackButton />
        <PipelineLayoutSidebar
          activeItem={getActiveItem()}
          onItemClick={handleItemClick}
        />
      </LeftColumn>
      <RightColumn>
        <PipelineLayoutNavbar.Content
          name={name}
          status={status}
          source={source}
          sinks={sinks}
          isEnabled={isEnabled}
          onToggleEnabled={setIsEnabled}
          onRun={handleRun}
        />
        <ContentWrapper>
          <ContentIsland>{children}</ContentIsland>
        </ContentWrapper>
      </RightColumn>
    </LayoutWrapper>
  );
};

export default PipelineLayout;
