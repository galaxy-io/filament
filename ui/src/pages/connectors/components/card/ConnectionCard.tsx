import { FlowArrowIcon } from "@phosphor-icons/react";
import pluralize from "pluralize";

import Chip, { ChipSize, ChipVariant } from "@galaxy-io/dls/chips/Chip";
import Flex, { AlignItems, JustifyContent } from "@galaxy-io/dls/layout/Flex";
import FlexItem from "@galaxy-io/dls/layout/FlexItem";
import Text, { TextSize, TextVariant, TextWeight } from "@galaxy-io/dls/text/Text";
import Widget from "@galaxy-io/dls/widget/Widget";

import type { Connection } from "@/gen/ingestion/v1/connections_pb";

import ConnectionKindChip from "@/pages/connectors/components/ConnectionKindChip";
import ConnectorTile from "@/pages/connectors/components/ConnectorTile";

interface ConnectionCardProps {
  connection: Connection;
  pipelineCount?: number;
  onClick: () => void;
}

const ConnectionCard = ({ connection, pipelineCount = 0, onClick }: ConnectionCardProps) => {
  const pipelineLabel = pluralize("pipeline", pipelineCount, true);

  return (
    <Widget
      isInteractive
      onClick={onClick}
      ariaLabel={connection.name}
      footer={
        <Text size={TextSize.BODY_SM} variant={TextVariant.SECONDARY}>
          Version {connection.version.toString()}
        </Text>
      }
    >
      <Flex alignItems={AlignItems.START} fillWidth justifyContent={JustifyContent.SPACE_BETWEEN}>
        <Flex alignItems={AlignItems.CENTER} gap={8}>
          <FlexItem shrink={0}>
            <ConnectorTile connector={connection.connector} kind={connection.kind} />
          </FlexItem>
          <Text size={TextSize.BODY_LG} weight={TextWeight.MEDIUM}>
            {connection.name}
          </Text>
        </Flex>
        <ConnectionKindChip kind={connection.kind} size={ChipSize.SMALL} />
      </Flex>
      <Flex alignItems={AlignItems.START}>
        <Chip icon={FlowArrowIcon} label={pipelineLabel} variant={ChipVariant.SECONDARY} />
      </Flex>
    </Widget>
  );
};

export default ConnectionCard;
