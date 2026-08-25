import { styled } from "@linaria/react";
import { PlusIcon } from "@phosphor-icons/react";

import Icon, { IconVariant } from "@galaxy-io/dls/icons/Icon";
import { withTheme } from "@galaxy-io/dls/theme/GalaxyTheme";
import type { PropsWithTheme } from "@galaxy-io/dls/theme/types";

import { ConnectorKind } from "@/gen/ingestion/v1/common_pb";

import EmptyGraphic, { EmptyGraphicGhostTile } from "@/components/EmptyGraphic";

import ConnectorTile from "@/pages/connectors/components/ConnectorTile";

import { useListConnectorsQuery } from "@/api/queries/connectors";

const SLOT_COLUMN_COUNT = 9;
const SLOT_COUNT = 27;
const SLOT_LIVE_INDEX = (SLOT_COUNT - 1) / 2;
const SLOT_SIZE = 44;

const SLOTS = Array.from({ length: SLOT_COUNT }, (_, index) => index);

const SlotsWrapper = styled.div`
  display: grid;
  grid-template-columns: repeat(${SLOT_COLUMN_COUNT}, ${SLOT_SIZE}px);
  gap: 12px;

  mask-image: linear-gradient(180deg, black 55%, transparent 110%);
  -webkit-mask-image: linear-gradient(180deg, black 55%, transparent 110%);
`;

const Slot = withTheme(styled.div<PropsWithTheme<{ $isLive: boolean; $isFilled: boolean }>>`
  width: ${SLOT_SIZE}px;
  height: ${SLOT_SIZE}px;

  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;

  border: ${({ $isFilled }) => ($isFilled ? "0.5px solid" : "1px dashed")}
    ${({ theme, $isLive }) =>
      $isLive ? theme.color.border.secondary : theme.color.border.primary};
  border-radius: 6px;

  animation: ${({ $isLive }) =>
    $isLive ? "sources-empty-slot-pulse 3s ease-in-out infinite" : "none"};

  @keyframes sources-empty-slot-pulse {
    0%,
    100% {
      opacity: 0.55;
    }
    50% {
      opacity: 1;
    }
  }
`);

const ConnectionsPageSourcesEmptyGraphic = () => {
  const { data } = useListConnectorsQuery();
  const sourceSpecs = (data?.connectors ?? []).filter(
    (connector) => connector.kind === ConnectorKind.SOURCE,
  );

  return (
    <EmptyGraphic>
      <SlotsWrapper>
        {SLOTS.map((slot) => {
          if (slot === SLOT_LIVE_INDEX) {
            return (
              <Slot key={slot} $isLive $isFilled={false}>
                <Icon component={PlusIcon} size={16} variant={IconVariant.SECONDARY} />
              </Slot>
            );
          }

          const row = Math.floor(slot / SLOT_COLUMN_COUNT);
          const spec = sourceSpecs.length
            ? sourceSpecs[(slot + row * 4) % sourceSpecs.length]
            : undefined;

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
