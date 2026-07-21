import { styled } from "@linaria/react";
import { FlowArrowIcon, InfoIcon } from "@phosphor-icons/react";
import pluralize from "pluralize";

import Chip, { ChipSize, ChipVariant } from "@galaxy-io/dls/chips/Chip";
import FlexItem from "@galaxy-io/dls/containers/FlexItem";
import FlexWrapper, {
  AlignItems,
  FlexDirection,
  JustifyContent,
} from "@galaxy-io/dls/containers/FlexWrapper";
import HorizontalDivider from "@galaxy-io/dls/dividers/HorizontalDivider";
import Text, {
  TextSize,
  TextVariant,
  TextWeight,
} from "@galaxy-io/dls/text/Text";
import { withTheme } from "@galaxy-io/dls/theme/GalaxyTheme";
import type { PropsWithTheme } from "@galaxy-io/dls/theme/types";

import ConnectorTile from "@/pages/connectors/components/ConnectorTile";

import { ConnectorKind } from "@/gen/ingestion/v1/common_pb";
import type { Connection } from "@/gen/ingestion/v1/connections_pb";
import Tooltip, { TooltipPosition } from "@galaxy-io/dls/tooltip/Tooltip";
import Icon, { IconVariant } from "@galaxy-io/dls/icons/Icon";

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

const ConnectionCard = ({
  connection,
  pipelineCount = 0,
  onClick,
}: ConnectionCardProps) => {
  const isSource = connection.kind === ConnectorKind.SOURCE;
  const kindLabel = isSource ? "Source" : "Sink";
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
          <Chip
            label={kindLabel}
            variant={isSource ? ChipVariant.LIME : ChipVariant.PINK}
            size={ChipSize.SMALL}
          />
        </FlexWrapper>
        <FlexItem shrink={0}>
          <Chip
            icon={FlowArrowIcon}
            label={pipelineLabel}
            variant={ChipVariant.TERTIARY}
          />
        </FlexItem>
      </FlexWrapper>
      <HorizontalDivider />
      <FlexWrapper alignItems={AlignItems.CENTER} justifyContent={JustifyContent.SPACE_BETWEEN} gap={12} padding={"12px"}>
        <Text size={TextSize.BODY_SM} variant={TextVariant.SECONDARY}>
          Version {connection.version.toString()}
        </Text>
        <Tooltip
          body={
            <Text size={TextSize.CAPTION} isMonospace isSelectable>
              {connection.id}
            </Text>
          }
          position={TooltipPosition.LEFT}
          isInteractive
        >
          <Icon component={InfoIcon} variant={IconVariant.TERTIARY} size={14} />
        </Tooltip>
      </FlexWrapper>
    </CardWrapper>
  );
};

export default ConnectionCard;
