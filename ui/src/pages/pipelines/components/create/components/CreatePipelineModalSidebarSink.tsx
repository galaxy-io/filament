import { WarningIcon } from "@phosphor-icons/react";

import FlexItem from "@galaxy-io/dls/containers/FlexItem";
import FlexWrapper, { AlignItems, JustifyContent } from "@galaxy-io/dls/containers/FlexWrapper";
import Icon, { IconVariant, IconWeight } from "@galaxy-io/dls/icons/Icon";
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
    <FlexWrapper
      alignItems={AlignItems.CENTER}
      justifyContent={JustifyContent.SPACE_BETWEEN}
      gap={8}
      padding="4px 6px"
      margin="0 0 0 18px"
      minWidth={0}
      onClick={onClick}
    >
      <FlexWrapper alignItems={AlignItems.CENTER} gap={8} minWidth={0} fillWidth>
        <ConnectorTile
          connector={sink.connection.connector}
          kind={sink.connection.kind}
          size={ConnectorTileSize.SMALL}
        />
        <FlexItem minWidth={0} overflow="hidden">
          <Text
            size={TextSize.BODY_SM}
            variant={isActive ? TextVariant.PRIMARY : TextVariant.TERTIARY}
            isEllipsis
          >
            {sink.connection.name}
          </Text>
        </FlexItem>
      </FlexWrapper>
      {issues.length > 0 && (
        <Icon component={WarningIcon} variant={IconVariant.ERROR} weight={IconWeight.FILL} />
      )}
    </FlexWrapper>
  );
};

export default CreatePipelineModalSidebarSink;
