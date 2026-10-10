import type { FC } from "react";

import { styled } from "@linaria/react";
import { PlusIcon } from "@phosphor-icons/react";

import Icon, { IconVariant } from "@galaxy-io/dls/icons/Icon";
import { HAIRLINE_WIDTH, REDUCED_MOTION } from "@galaxy-io/dls/styles/mixins";
import { t } from "@galaxy-io/dls/theme/tokens/t";

import { ConnectorKind } from "@/gen/ingestion/v1/common_pb";

import ConnectorTile from "@/components/connections/ConnectorTile";
import { useConnectorFamilies } from "@/components/connections/hooks/useConnectorFamilies";
import EmptyGraphic from "@/components/EmptyGraphic";
import EmptyGraphicGhostTile from "@/components/EmptyGraphicGhostTile";

const SOURCES_EMPTY_SLOT_COLUMN_COUNT = 9;
const SOURCES_EMPTY_SLOT_COUNT = 27;
const SOURCES_EMPTY_SLOT_LIVE_INDEX = (SOURCES_EMPTY_SLOT_COUNT - 1) / 2;
const SOURCES_EMPTY_SLOT_SIZE = 44;
const SOURCES_EMPTY_SLOT_PULSE_DURATION = "3s";

const SOURCES_EMPTY_SLOTS = Array.from({ length: SOURCES_EMPTY_SLOT_COUNT }, (_, index) => index);

const SlotsWrapper = styled.div`
  display: grid;
  grid-template-columns: repeat(${SOURCES_EMPTY_SLOT_COLUMN_COUNT}, ${SOURCES_EMPTY_SLOT_SIZE}px);
  gap: ${t.space[12]};

  mask-image: linear-gradient(180deg, black 55%, transparent 110%);
  -webkit-mask-image: linear-gradient(180deg, black 55%, transparent 110%);
`;

const Slot = styled.div<{ $isLive: boolean; $isFilled: boolean }>`
  width: ${SOURCES_EMPTY_SLOT_SIZE}px;
  height: ${SOURCES_EMPTY_SLOT_SIZE}px;

  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;

  border: ${({ $isFilled }) => ($isFilled ? `${HAIRLINE_WIDTH} solid` : `${HAIRLINE_WIDTH} dashed`)}
    ${({ $isLive }) => ($isLive ? t.color.border.secondary : t.color.border.primary)};
  border-radius: ${t.radius.md};

  animation: ${({ $isLive }) =>
    $isLive
      ? `sources-empty-slot-pulse ${SOURCES_EMPTY_SLOT_PULSE_DURATION} ease-in-out infinite`
      : "none"};

  @keyframes sources-empty-slot-pulse {
    0%,
    100% {
      opacity: 0.55;
    }
    50% {
      opacity: 1;
    }
  }

  ${REDUCED_MOTION}
`;

const ConnectionsPageSourcesEmptyGraphic: FC = () => {
  const sourceSpecs = useConnectorFamilies(ConnectorKind.SOURCE);

  return (
    <EmptyGraphic>
      <SlotsWrapper>
        {SOURCES_EMPTY_SLOTS.map((slot) => {
          if (slot === SOURCES_EMPTY_SLOT_LIVE_INDEX) {
            return (
              <Slot key={slot} $isLive $isFilled={false}>
                <Icon component={PlusIcon} size={16} variant={IconVariant.SECONDARY} />
              </Slot>
            );
          }

          const spec = sourceSpecs[slot < SOURCES_EMPTY_SLOT_LIVE_INDEX ? slot : slot - 1];

          return (
            <Slot key={slot} $isLive={false} $isFilled={!!spec}>
              {spec && (
                <EmptyGraphicGhostTile>
                  <ConnectorTile connector={spec.name} kind={spec.kind} />
                </EmptyGraphicGhostTile>
              )}
            </Slot>
          );
        })}
      </SlotsWrapper>
    </EmptyGraphic>
  );
};

export default ConnectionsPageSourcesEmptyGraphic;
