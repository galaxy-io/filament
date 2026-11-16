import type { FC } from "react";

import { styled } from "@linaria/react";
import pluralize from "pluralize";

import Beacon, { BeaconSize } from "@galaxy-io/dls/beacons/Beacon";
import FlexItem from "@galaxy-io/dls/layout/FlexItem";
import { FOCUS_RING, HAIRLINE_WIDTH, INTERACTIVE_RESET } from "@galaxy-io/dls/styles/mixins";
import Text, { TextSize, TextVariant } from "@galaxy-io/dls/text/Text";
import { t } from "@galaxy-io/dls/theme/tokens/t";

import type { Connection } from "@/gen/ingestion/v1/connections_pb";

import ConnectorTile from "@/components/connections/ConnectorTile";

import type { ObservabilityConnectionStats } from "@/pages/observability/components/connections/types";
import { OBSERVABILITY_LIST_ROW_HEIGHT } from "@/pages/observability/constants";

import { useFilamentSearchUpdate } from "@/module/hooks";
import type { FilamentLayoutSearch } from "@/module/schemas";

const ObservabilityConnectionRowButton = styled.button`
  ${INTERACTIVE_RESET}
  ${FOCUS_RING}
  width: 100%;
  height: ${OBSERVABILITY_LIST_ROW_HEIGHT}px;
  padding: 0 ${t.space[12]};
  display: flex;
  align-items: center;
  gap: ${t.space[8]};
  border-bottom: ${HAIRLINE_WIDTH} solid ${t.color.border.primary};
  transition: background-color ${t.duration.fast};

  &:hover {
    background-color: ${t.color.background.hovered};
  }
`;

interface ObservabilityConnectionRowProps {
  connection: Connection;
  stats: ObservabilityConnectionStats | undefined;
}

const ObservabilityConnectionRow: FC<ObservabilityConnectionRowProps> = ({ connection, stats }) => {
  const updateSearch = useFilamentSearchUpdate<FilamentLayoutSearch>();
  const pipelineCount = stats?.pipelineCount ?? 0;

  const handleClick = () => {
    void updateSearch((prev) => ({ ...prev, connectionId: connection.id }));
  };

  return (
    <ObservabilityConnectionRowButton type="button" onClick={handleClick}>
      <ConnectorTile connector={connection.connector} kind={connection.kind} />
      <FlexItem grow={1} minWidth={0}>
        <Text
          variant={pipelineCount > 0 ? TextVariant.PRIMARY : TextVariant.TERTIARY}
          lineClamp={1}
        >
          {connection.name}
        </Text>
      </FlexItem>
      {stats?.isLive && <Beacon color="blue" size={BeaconSize.SMALL} isPulse ariaLabel="Running" />}
      <Text size={TextSize.BODY_SM} variant={TextVariant.TERTIARY}>
        {pluralize("pipeline", pipelineCount, true)}
      </Text>
    </ObservabilityConnectionRowButton>
  );
};

export default ObservabilityConnectionRow;
