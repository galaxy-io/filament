import { styled } from "@linaria/react";
import { PlusIcon } from "@phosphor-icons/react";

import Icon, { IconVariant } from "@galaxy-io/dls/icons/Icon";
import FlexItem from "@galaxy-io/dls/layout/FlexItem";
import { HAIRLINE_WIDTH } from "@galaxy-io/dls/styles/mixins";
import { t } from "@galaxy-io/dls/theme/tokens/t";

import { ConnectorKind } from "@/gen/ingestion/v1/common_pb";

import EmptyGraphic, {
  EmptyGraphicGhostBar,
  EmptyGraphicGhostTile,
  EmptyGraphicGhostTileFallback,
} from "@/components/EmptyGraphic";

import ConnectorTile from "@/pages/connectors/components/ConnectorTile";

import { useListConnectorsQuery } from "@/api/queries/connectors";

interface SinkEmptyCard {
  nameWidth: number;
  metaWidth: number;
}

const SINKS_EMPTY_CARD_HEIGHT = 92;

const SINK_EMPTY_CARDS: SinkEmptyCard[] = [
  { nameWidth: 72, metaWidth: 56 },
  { nameWidth: 56, metaWidth: 44 },
  { nameWidth: 64, metaWidth: 50 },
  { nameWidth: 70, metaWidth: 60 },
  { nameWidth: 50, metaWidth: 40 },
];

const CardsWrapper = styled.div`
  width: 100%;

  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 12px;

  mask-image: linear-gradient(180deg, black 55%, transparent 110%);
  -webkit-mask-image: linear-gradient(180deg, black 55%, transparent 110%);
`;

const GhostCard = styled.div`
  height: ${SINKS_EMPTY_CARD_HEIGHT}px;
  min-width: 0;

  display: flex;
  flex-direction: column;
  gap: 12px;
  padding: 12px;

  background-color: ${t.color.background.primary};

  border: ${HAIRLINE_WIDTH} solid ${t.color.border.primary};
  border-radius: ${t.radius.lg};
`;

const LiveCard = styled.div`
  height: ${SINKS_EMPTY_CARD_HEIGHT}px;
  min-width: 0;

  display: flex;
  align-items: center;
  justify-content: center;

  border: 1px dashed ${t.color.border.secondary};
  border-radius: ${t.radius.lg};
`;

const GhostCardRow = styled.div`
  min-width: 0;

  display: flex;
  align-items: center;
  gap: 8px;
`;

const GhostChip = styled.div`
  width: 32px;
  height: 12px;

  background-color: ${t.color.background.secondary};

  border-radius: ${t.radius.sm};
`;

const ConnectionsPageSinksEmptyGraphic = () => {
  const { data } = useListConnectorsQuery();
  const sinkSpecs = (data?.connectors ?? []).filter(
    (connector) => connector.kind === ConnectorKind.SINK,
  );

  return (
    <EmptyGraphic>
      <CardsWrapper>
        <LiveCard>
          <Icon component={PlusIcon} size={16} variant={IconVariant.SECONDARY} />
        </LiveCard>
        {SINK_EMPTY_CARDS.map((card, index) => {
          const spec = sinkSpecs.length ? sinkSpecs[index % sinkSpecs.length] : undefined;

          return (
            <GhostCard key={card.nameWidth}>
              <GhostCardRow>
                {spec ? (
                  <EmptyGraphicGhostTile>
                    <ConnectorTile connector={spec.name} kind={spec.kind} />
                  </EmptyGraphicGhostTile>
                ) : (
                  <EmptyGraphicGhostTileFallback />
                )}
                <EmptyGraphicGhostBar $width={card.nameWidth} />
                <FlexItem grow={1} />
                <GhostChip />
              </GhostCardRow>
              <EmptyGraphicGhostBar $width={card.metaWidth} />
            </GhostCard>
          );
        })}
      </CardsWrapper>
    </EmptyGraphic>
  );
};

export default ConnectionsPageSinksEmptyGraphic;
