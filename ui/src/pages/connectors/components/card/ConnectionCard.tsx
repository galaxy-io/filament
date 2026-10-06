import { styled } from "@linaria/react";
import { FlowArrowIcon } from "@phosphor-icons/react";
import pluralize from "pluralize";

import Chip, { ChipSize, ChipVariant } from "@galaxy-io/dls/chips/Chip";
import Divider from "@galaxy-io/dls/layout/Divider";
import Flex, { AlignItems, FlexDirection, JustifyContent } from "@galaxy-io/dls/layout/Flex";
import FlexItem from "@galaxy-io/dls/layout/FlexItem";
import Text, { TextSize, TextVariant, TextWeight } from "@galaxy-io/dls/text/Text";
import { t } from "@galaxy-io/dls/theme/tokens/t";

import type { Connection } from "@/gen/ingestion/v1/connections_pb";

import ConnectionKindChip from "@/pages/connectors/components/ConnectionKindChip";
import ConnectorTile from "@/pages/connectors/components/ConnectorTile";

const CardWrapper = styled.div`
  width: 100%;

  display: flex;
  flex-direction: column;

  background-color: ${t.color.background.primary};

  border: 0.5px solid ${t.color.border.primary};
  border-radius: ${t.radius.lg};

  cursor: pointer;

  transition: border-color 100ms ease;

  &:hover {
    border-color: ${t.color.border.secondary};
  }
`;

interface ConnectionCardProps {
  connection: Connection;
  pipelineCount?: number;
  onClick?: () => void;
}

const ConnectionCard = ({ connection, pipelineCount = 0, onClick }: ConnectionCardProps) => {
  const pipelineLabel = pluralize("pipeline", pipelineCount, true);

  return (
    <CardWrapper onClick={onClick}>
      <Flex alignItems={AlignItems.START} direction={FlexDirection.COLUMN} gap={12} padding={12}>
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
        <FlexItem shrink={0}>
          <Chip icon={FlowArrowIcon} label={pipelineLabel} variant={ChipVariant.SECONDARY} />
        </FlexItem>
      </Flex>
      <Divider />
      <Flex alignItems={AlignItems.START} padding={12}>
        <Text size={TextSize.BODY_SM} variant={TextVariant.SECONDARY}>
          Version {connection.version.toString()}
        </Text>
      </Flex>
    </CardWrapper>
  );
};

export default ConnectionCard;
