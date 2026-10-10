import type { FC } from "react";

import { styled } from "@linaria/react";
import { PlusIcon } from "@phosphor-icons/react";

import Icon, { IconVariant } from "@galaxy-io/dls/icons/Icon";
import Box, { BoxVariant } from "@galaxy-io/dls/layout/Box";
import Flex, { AlignItems, FlexDirection } from "@galaxy-io/dls/layout/Flex";
import FlexItem from "@galaxy-io/dls/layout/FlexItem";
import { Radius } from "@galaxy-io/dls/theme/enums";
import { t } from "@galaxy-io/dls/theme/tokens/t";

import { ConnectorKind } from "@/gen/ingestion/v1/common_pb";

import EmptyGraphic, {
  EmptyGraphicGhostBar,
  EmptyGraphicGhostTile,
  EmptyGraphicGhostTileFallback,
  useEmptyGraphicConnectors,
} from "@/components/EmptyGraphic";

import ConnectorTile from "@/pages/connectors/components/ConnectorTile";

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

const LiveCard = styled.div`
  height: ${SINKS_EMPTY_CARD_HEIGHT}px;
  min-width: 0;

  display: flex;
  align-items: center;
  justify-content: center;

  border: 1px dashed ${t.color.border.secondary};
  border-radius: ${t.radius.lg};
`;

const ConnectionsPageSinksEmptyGraphic: FC = () => {
  const sinkSpecs = useEmptyGraphicConnectors(ConnectorKind.SINK);

  return (
    <EmptyGraphic>
      <CardsWrapper>
        <LiveCard>
          <Icon component={PlusIcon} size={16} variant={IconVariant.SECONDARY} />
        </LiveCard>
        {SINK_EMPTY_CARDS.map((card, index) => {
          const spec = sinkSpecs[index];

          return (
            <Box
              key={card.nameWidth}
              height={SINKS_EMPTY_CARD_HEIGHT}
              minWidth={0}
              variant={BoxVariant.PRIMARY}
              hasBorder
              radius={Radius.LG}
              padding={12}
            >
              <Flex direction={FlexDirection.COLUMN} gap={12}>
                <Flex alignItems={AlignItems.CENTER} gap={8} minWidth={0}>
                  {spec ? (
                    <EmptyGraphicGhostTile>
                      <ConnectorTile connector={spec.name} kind={spec.kind} />
                    </EmptyGraphicGhostTile>
                  ) : (
                    <EmptyGraphicGhostTileFallback />
                  )}
                  <EmptyGraphicGhostBar $width={card.nameWidth} />
                  <FlexItem grow={1} />
                  <Box width={32} height={12} variant={BoxVariant.SECONDARY} radius={Radius.SM} />
                </Flex>
                <EmptyGraphicGhostBar $width={card.metaWidth} />
              </Flex>
            </Box>
          );
        })}
      </CardsWrapper>
    </EmptyGraphic>
  );
};

export default ConnectionsPageSinksEmptyGraphic;
