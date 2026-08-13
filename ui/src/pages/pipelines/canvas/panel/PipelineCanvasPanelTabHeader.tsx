import { styled } from "@linaria/react";
import { XIcon } from "@phosphor-icons/react";

import Button, { ButtonSize, ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import FlexItem from "@galaxy-io/dls/containers/FlexItem";
import FlexWrapper, { AlignItems, JustifyContent } from "@galaxy-io/dls/containers/FlexWrapper";
import HorizontalDivider from "@galaxy-io/dls/dividers/HorizontalDivider";
import Text, { TextVariant } from "@galaxy-io/dls/text/Text";
import { withTheme } from "@galaxy-io/dls/theme/GalaxyTheme";
import type { PropsWithTheme } from "@galaxy-io/dls/theme/types";

import { usePipelineCanvasSelection } from "@/pages/pipelines/canvas/hooks/usePipelineCanvasSelection";
import { PIPELINE_CANVAS_PANEL_HEADER_HEIGHT } from "@/pages/pipelines/canvas/panel/constants";
import { PipelineCanvasPanelTab } from "@/pages/pipelines/canvas/panel/types";

const PIPELINE_CANVAS_PANEL_TAB_TO_LABEL_MAP: Record<PipelineCanvasPanelTab, string> = {
  [PipelineCanvasPanelTab.OVERVIEW]: "Overview",
  [PipelineCanvasPanelTab.ACTIVITY]: "Activity",
};

const TabsWrapper = styled.div`
  height: ${PIPELINE_CANVAS_PANEL_HEADER_HEIGHT}px;

  display: flex;
  align-items: stretch;
  gap: 6px;
  flex-shrink: 0;
`;

const TabButton = withTheme(styled.button<PropsWithTheme<{ $isActive: boolean }>>`
  display: flex;
  align-items: center;
  flex-shrink: 0;

  padding: 0 10px;

  opacity: ${({ $isActive }) => ($isActive ? 1 : 0.5)};

  border: none;
  border-bottom: 2px solid
    ${({ theme, $isActive }) => ($isActive ? theme.color.background.primaryAlt : "transparent")};
  background-color: transparent;
  cursor: pointer;

  transition:
    opacity 100ms ease,
    border-color 100ms ease;

  &:hover {
    opacity: ${({ $isActive }) => ($isActive ? 1 : 0.75)};
  }
`);

const PipelineCanvasPanelTabHeader = () => {
  const { activeTab, setActiveTab, setShowPanel } = usePipelineCanvasSelection();

  return (
    <>
      <FlexWrapper
        alignItems={AlignItems.CENTER}
        justifyContent={JustifyContent.SPACE_BETWEEN}
        gap={8}
        padding="0 12px"
        shrink={0}
        fillWidth
      >
        <TabsWrapper>
          {Object.values(PipelineCanvasPanelTab).map((tab) => (
            <TabButton
              key={tab}
              type="button"
              $isActive={tab === activeTab}
              onClick={() => setActiveTab(tab)}
            >
              <Text variant={TextVariant.PRIMARY}>
                {PIPELINE_CANVAS_PANEL_TAB_TO_LABEL_MAP[tab]}
              </Text>
            </TabButton>
          ))}
        </TabsWrapper>
        <FlexItem shrink={0}>
          <Button
            icon={XIcon}
            variant={ButtonVariant.SECONDARY}
            size={ButtonSize.SMALL}
            onClick={() => setShowPanel(false)}
            ariaLabel="Close configuration panel"
          />
        </FlexItem>
      </FlexWrapper>
      <HorizontalDivider />
    </>
  );
};

export default PipelineCanvasPanelTabHeader;
