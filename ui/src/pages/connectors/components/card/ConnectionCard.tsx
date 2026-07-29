import { styled } from "@linaria/react";
import { FlowArrowIcon } from "@phosphor-icons/react";
import pluralize from "pluralize";

import Chip, { ChipSize, ChipVariant } from "@galaxy-io/dls/chips/Chip";
import FlexItem from "@galaxy-io/dls/containers/FlexItem";
import FlexWrapper, {
  AlignItems,
  FlexDirection,
  JustifyContent,
} from "@galaxy-io/dls/containers/FlexWrapper";
import HorizontalDivider from "@galaxy-io/dls/dividers/HorizontalDivider";
import Text, { TextSize, TextVariant, TextWeight } from "@galaxy-io/dls/text/Text";
import { withTheme } from "@galaxy-io/dls/theme/GalaxyTheme";
import type { PropsWithTheme } from "@galaxy-io/dls/theme/types";

import type { Connection } from "@/gen/ingestion/v1/connections_pb";

import ConnectionKindChip from "@/pages/connectors/components/ConnectionKindChip";
import ConnectorTile from "@/pages/connectors/components/ConnectorTile";

const CardWrapper = withTheme(styled.div<PropsWithTheme>`
  width: 100%;

  display: flex;
  flex-direction: column;

  background-color: ${({ theme }) => theme.color.background.primary};

  border: 0.5px solid ${({ theme }) => theme.color.border.primary};
  border-radius: 5px;

  cursor: pointer;

  transition: border-color 100ms ease;

  &:hover {
    border-color: ${({ theme }) => theme.color.border.secondary};
  }
`);

interface ConnectionCardProps {
  connection: Connection;
  pipelineCount?: number;
  onClick?: () => void;
}

const ConnectionCard = ({ connection, pipelineCount = 0, onClick }: ConnectionCardProps) => {
  const pipelineLabel = pluralize("pipeline", pipelineCount, true);

  return (
    <CardWrapper onClick={onClick}>
      <FlexWrapper direction={FlexDirection.COLUMN} gap={12} padding={"12px"}>
        <FlexWrapper fillWidth justifyContent={JustifyContent.SPACE_BETWEEN}>
          <FlexWrapper alignItems={AlignItems.CENTER} gap={8}>
            <FlexItem shrink={0}>
              <ConnectorTile connector={connection.connector} />
            </FlexItem>
            <Text size={TextSize.BODY_LG} weight={TextWeight.MEDIUM}>
              {connection.name}
            </Text>
          </FlexWrapper>
          <ConnectionKindChip kind={connection.kind} size={ChipSize.SMALL} />
        </FlexWrapper>
        <FlexItem shrink={0}>
          <Chip icon={FlowArrowIcon} label={pipelineLabel} variant={ChipVariant.SECONDARY} />
        </FlexItem>
      </FlexWrapper>
      <HorizontalDivider />
      <FlexWrapper padding={"12px"}>
        <Text size={TextSize.BODY_SM} variant={TextVariant.SECONDARY}>
          Version {connection.version.toString()}
        </Text>
      </FlexWrapper>
    </CardWrapper>
  );
};

export default ConnectionCard;
