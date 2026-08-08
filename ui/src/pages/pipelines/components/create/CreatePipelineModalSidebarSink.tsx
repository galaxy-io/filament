import { styled } from "@linaria/react";
import { WarningIcon } from "@phosphor-icons/react";

import FlexWrapper, { AlignItems } from "@galaxy-io/dls/containers/FlexWrapper";
import Icon, { IconVariant, IconWeight } from "@galaxy-io/dls/icons/Icon";
import Text, { TextSize, TextVariant } from "@galaxy-io/dls/text/Text";
import { withTheme } from "@galaxy-io/dls/theme/GalaxyTheme";
import type { PropsWithTheme } from "@galaxy-io/dls/theme/types";

import ConnectorTile, { ConnectorTileSize } from "@/pages/connectors/components/ConnectorTile";
import type { CreatePipelineModalSinkRow } from "@/pages/pipelines/components/create/types";

const SinkButton = withTheme(styled.button<PropsWithTheme<{ $isActive: boolean }>>`
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  min-width: 0;

  padding: 4px 6px;
  margin-left: 18px;

  border: none;
  border-radius: 4px;
  cursor: pointer;
`);

const SinkName = styled.div`
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
`;

interface CreatePipelineModalSidebarSinkProps {
  sink: CreatePipelineModalSinkRow;
  isActive: boolean;
  hasIssue: boolean;
  onClick?: () => void;
}

const CreatePipelineModalSidebarSink = ({
  sink,
  isActive,
  hasIssue,
  onClick,
}: CreatePipelineModalSidebarSinkProps) => {
  return (
    <SinkButton $isActive={isActive} onClick={onClick}>
      <FlexWrapper alignItems={AlignItems.CENTER} gap={8} fillWidth>
        <ConnectorTile connector={sink.connection.connector} size={ConnectorTileSize.SMALL} />
        <SinkName>
          <Text
            size={TextSize.BODY_SM}
            variant={isActive ? TextVariant.PRIMARY : TextVariant.TERTIARY}
          >
            {sink.connection.name}
          </Text>
        </SinkName>
      </FlexWrapper>
      {hasIssue && (
        <Icon component={WarningIcon} variant={IconVariant.ERROR} weight={IconWeight.FILL} />
      )}
    </SinkButton>
  );
};

export default CreatePipelineModalSidebarSink;
