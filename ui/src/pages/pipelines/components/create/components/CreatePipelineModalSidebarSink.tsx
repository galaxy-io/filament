import { styled } from "@linaria/react";
import { WarningIcon } from "@phosphor-icons/react";

import Icon, { IconVariant, IconWeight } from "@galaxy-io/dls/icons/Icon";
import Flex, { AlignItems } from "@galaxy-io/dls/layout/Flex";
import FlexItem from "@galaxy-io/dls/layout/FlexItem";
import Text, { TextSize, TextVariant } from "@galaxy-io/dls/text/Text";
import { t } from "@galaxy-io/dls/theme/tokens/t";

import ConnectorTile, { ConnectorTileSize } from "@/pages/connectors/components/ConnectorTile";
import type { CreatePipelineModalSinkRow } from "@/pages/pipelines/components/create/types";

const SinkButton = styled.button`
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: ${t.space[8]};
  min-width: 0;
  width: 100%;
  padding: ${t.space[4]} ${t.space[8]};

  text-align: left;
  background-color: transparent;
  border: none;
  cursor: pointer;

  &:disabled {
    cursor: default;
  }
`;

interface CreatePipelineModalSidebarSinkProps {
  sink: CreatePipelineModalSinkRow;
  isActive: boolean;
  issues: string[];
  onClick?: () => void;
}

const CreatePipelineModalSidebarSink = ({
  sink,
  isActive,
  issues,
  onClick,
}: CreatePipelineModalSidebarSinkProps) => {
  return (
    <SinkButton type="button" onClick={onClick} disabled={!onClick}>
      <Flex alignItems={AlignItems.CENTER} gap={8} minWidth={0} fillWidth>
        <ConnectorTile
          connector={sink.connection.connector}
          kind={sink.connection.kind}
          size={ConnectorTileSize.SMALL}
        />
        <FlexItem minWidth={0} overflow="hidden">
          <Text
            size={TextSize.BODY_SM}
            variant={isActive ? TextVariant.PRIMARY : TextVariant.TERTIARY}
            lineClamp={1}
          >
            {sink.connection.name}
          </Text>
        </FlexItem>
      </Flex>
      {issues.length > 0 && (
        <Icon component={WarningIcon} variant={IconVariant.ERROR} weight={IconWeight.FILL} />
      )}
    </SinkButton>
  );
};

export default CreatePipelineModalSidebarSink;
