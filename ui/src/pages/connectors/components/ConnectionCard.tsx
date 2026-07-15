import { styled } from "@linaria/react";
import { FlowArrowIcon } from "@phosphor-icons/react";
import pluralize from "pluralize";

import Chip, { ChipVariant } from "@galaxy-io/dls/chips/Chip";
import FlexItem from "@galaxy-io/dls/containers/FlexItem";
import FlexWrapper, {
  AlignItems,
  FlexGap,
  JustifyContent,
} from "@galaxy-io/dls/containers/FlexWrapper";
import Text, { TextSize, TextWeight } from "@galaxy-io/dls/text/Text";
import { withTheme } from "@galaxy-io/dls/theme/GalaxyTheme";
import type { PropsWithTheme } from "@galaxy-io/dls/theme/types";

import ConnectorTile from "@/pages/connectors/components/ConnectorTile";

import { ConnectorKind } from "@/gen/ingestion/v1/common_pb";
import type { Connection } from "@/gen/ingestion/v1/connections_pb";

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

const CardSection = styled.div`
  display: flex;
  flex-direction: column;
  gap: 12px;

  padding: 16px;
`;

interface ConnectionCardProps {
  connection: Connection;
  pipelineCount?: number;
  onClick?: () => void;
}

const ConnectionCard = ({ connection, pipelineCount = 0, onClick }: ConnectionCardProps) => {
  const isSource = connection.kind === ConnectorKind.SOURCE;
  const kindLabel = isSource ? "Source" : "Sink";
  const pipelineLabel = pluralize("pipeline", pipelineCount, true);

  return (
    <CardWrapper onClick={onClick}>
      <CardSection>
        <FlexWrapper
          fillWidth
          alignItems={AlignItems.CENTER}
          justifyContent={JustifyContent.SPACE_BETWEEN}
        >
          <FlexWrapper alignItems={AlignItems.CENTER} gap={FlexGap.SMALL}>
            <FlexItem shrink={0}>
              <ConnectorTile connector={connection.connector} />
            </FlexItem>
            <Text size={TextSize.BODY_LG} weight={TextWeight.MEDIUM}>
              {connection.name}
            </Text>
          </FlexWrapper>
          <Chip label={kindLabel} variant={isSource ? ChipVariant.LIME : ChipVariant.PINK} />
        </FlexWrapper>
        <FlexItem shrink={0}>
          <Chip icon={FlowArrowIcon} label={pipelineLabel} variant={ChipVariant.TERTIARY} />
        </FlexItem>
      </CardSection>
    </CardWrapper>
  );
};

export default ConnectionCard;
