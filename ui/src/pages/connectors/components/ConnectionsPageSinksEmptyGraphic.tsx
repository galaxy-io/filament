import { styled } from "@linaria/react";
import { PlusIcon } from "@phosphor-icons/react";

import FlexItem from "@galaxy-io/dls/containers/FlexItem";
import Icon, { IconVariant } from "@galaxy-io/dls/icons/Icon";
import { withTheme } from "@galaxy-io/dls/theme/GalaxyTheme";
import type { PropsWithTheme } from "@galaxy-io/dls/theme/types";

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

const GhostCard = withTheme(styled.div<PropsWithTheme>`
  height: 92px;
  min-width: 0;

  display: flex;
  flex-direction: column;
  gap: 12px;
  padding: 12px;

  background-color: ${({ theme }) => theme.color.background.primary};

  border: 0.5px solid ${({ theme }) => theme.color.border.primary};
  border-radius: 5px;
`);

const LiveCard = withTheme(styled.div<PropsWithTheme>`
  height: 92px;
  min-width: 0;

  display: flex;
  align-items: center;
  justify-content: center;

  border: 1px dashed ${({ theme }) => theme.color.border.secondary};
  border-radius: 5px;
`);

const GhostCardRow = styled.div`
  min-width: 0;

  display: flex;
  align-items: center;
  gap: 8px;
`;

const GhostChip = withTheme(styled.div<PropsWithTheme>`
  width: 32px;
  height: 12px;

  background-color: ${({ theme }) => theme.color.background.secondary};
  border-radius: 4px;
`);

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
                    <ConnectorTile connector={spec.name} spec={spec} />
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
