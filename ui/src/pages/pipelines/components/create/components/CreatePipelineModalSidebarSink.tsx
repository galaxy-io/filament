import { WarningIcon } from "@phosphor-icons/react";

import Icon, { IconVariant, IconWeight } from "@galaxy-io/dls/icons/Icon";
import Flex, { AlignItems, JustifyContent } from "@galaxy-io/dls/layout/Flex";
import FlexItem from "@galaxy-io/dls/layout/FlexItem";
import Text, { TextSize, TextVariant } from "@galaxy-io/dls/text/Text";

import ConnectorTile, { ConnectorTileSize } from "@/pages/connectors/components/ConnectorTile";
import type { CreatePipelineModalSinkRow } from "@/pages/pipelines/components/create/types";

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
    <Flex
      alignItems={AlignItems.CENTER}
      justifyContent={JustifyContent.SPACE_BETWEEN}
      gap={8}
      /* @dls-migrate layout.off-scale: Pick a value on the space scale (or a CSS-order tuple of them). */ padding="4px 6px"
      /* @dls-migrate flex.margin: Margins are forbidden: use the parent's `gap` / `padding`. */ margin="0 0 0 18px"
      minWidth={0}
      /* @dls-migrate flex.onClick: Add a `Button` or `Link` for keyboard users. */ onClick={
        onClick
      }
    >
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
    </Flex>
  );
};

export default CreatePipelineModalSidebarSink;
