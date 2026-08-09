import { styled } from "@linaria/react";
import { WarningIcon } from "@phosphor-icons/react";

import Badge, { BadgeSize, BadgeVariant } from "@galaxy-io/dls/badge/Badge";
import FlexWrapper, { AlignItems } from "@galaxy-io/dls/containers/FlexWrapper";
import Icon, { IconVariant, IconWeight } from "@galaxy-io/dls/icons/Icon";
import Text, { TextSize, TextVariant } from "@galaxy-io/dls/text/Text";
import { withTheme } from "@galaxy-io/dls/theme/GalaxyTheme";
import type { PropsWithTheme } from "@galaxy-io/dls/theme/types";

import ConnectorTile from "@/pages/connectors/components/ConnectorTile";
import type { CreatePipelineModalSinkRow } from "@/pages/pipelines/components/create/types";

const TabsWrapper = styled.div`
  display: flex;
  align-items: center;
  gap: 4px;
  flex-shrink: 0;

  padding: 4px 8px 0;
  overflow-x: auto;
`;

const TabButton = withTheme(styled.button<PropsWithTheme<{ $isActive: boolean }>>`
  display: flex;
  align-items: center;
  gap: 8px;
  flex-shrink: 0;

  padding: 8px 10px;

  opacity: ${({ $isActive }) => ($isActive ? 1 : 0.6)};

  border-bottom: 2px solid
    ${({ theme, $isActive }) => ($isActive ? theme.color.background.primaryAlt : "transparent")};
  cursor: pointer;
`);

interface CreatePipelineModalResourcesTabsProps {
  sinks: CreatePipelineModalSinkRow[];
  activeSinkId: string;
  selectedCountBySink: Record<string, number>;
  issuesBySink: Record<string, string[]>;
  onSelect: (sinkId: string) => void;
}

const CreatePipelineModalResourcesTabs = ({
  sinks,
  activeSinkId,
  selectedCountBySink,
  issuesBySink,
  onSelect,
}: CreatePipelineModalResourcesTabsProps) => {
  return (
    <TabsWrapper>
      {sinks.map((sink) => {
        const isActive = sink.connection.id === activeSinkId;
        const count = selectedCountBySink[sink.connection.id] ?? 0;
        const issues = issuesBySink[sink.connection.id] ?? [];

        return (
          <TabButton
            key={sink.connection.id}
            $isActive={isActive}
            onClick={() => onSelect(sink.connection.id)}
          >
            <ConnectorTile connector={sink.connection.connector} />
            <FlexWrapper alignItems={AlignItems.CENTER} gap={12}>
              <Text
                size={TextSize.BODY_SM}
                variant={TextVariant.PRIMARY}
              >
                {sink.connection.name}
              </Text>
              {issues.length ? (
                <Icon
                  component={WarningIcon}
                  variant={IconVariant.ERROR}
                  weight={IconWeight.FILL}
                />
              ) : (
                <Badge count={count} variant={BadgeVariant.SECONDARY} size={BadgeSize.SMALL} />
              )}
            </FlexWrapper>
          </TabButton>
        );
      })}
    </TabsWrapper>
  );
};

export default CreatePipelineModalResourcesTabs;
