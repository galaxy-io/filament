import { useMemo } from "react";

import { styled } from "@linaria/react";

import { withTheme } from "@galaxy-io/dls/theme/GalaxyTheme";
import type { PropsWithTheme } from "@galaxy-io/dls/theme/types";

import type { ConnectorKind } from "@/gen/ingestion/v1/common_pb";
import type { ConnectorSpec } from "@/gen/ingestion/v1/connectors_pb";

import { getConnectorFamily } from "@/pages/connectors/components/create/utils";

import { useListConnectorsQuery } from "@/api/queries/connectors";

export const EMPTY_GRAPHIC_WIDTH = 560;
export const EMPTY_GRAPHIC_HEIGHT = 200;

const EmptyGraphic = styled.div`
  width: ${EMPTY_GRAPHIC_WIDTH}px;
  height: ${EMPTY_GRAPHIC_HEIGHT}px;

  display: flex;
  align-items: center;
  justify-content: center;
`;

export const EmptyGraphicGhostTile = styled.div`
  display: flex;
  opacity: 0.3;
`;

export const EmptyGraphicGhostTileFallback = withTheme(styled.div<PropsWithTheme>`
  width: 24px;
  height: 24px;

  background-color: ${({ theme }) => theme.color.background.secondary};

  border: 0.5px solid ${({ theme }) => theme.color.border.primary};
  border-radius: 4px;
`);

export const EmptyGraphicGhostBar = withTheme(styled.div<PropsWithTheme<{ $width: number }>>`
  width: ${({ $width }) => $width}px;
  height: 12px;

  background-color: ${({ theme }) => theme.color.background.secondary};

  border-radius: 4px;
`);

const shuffleConnectors = (specs: ConnectorSpec[]) => {
  const shuffled = [...specs];
  for (let index = shuffled.length - 1; index > 0; index -= 1) {
    const swap = Math.floor(Math.random() * (index + 1));
    [shuffled[index], shuffled[swap]] = [shuffled[swap], shuffled[index]];
  }
  return shuffled;
};

export const useEmptyGraphicConnectors = (kind: ConnectorKind) => {
  const { data } = useListConnectorsQuery();

  return useMemo(() => {
    const catalog = data?.connectors ?? [];
    const families = new Map<string, ConnectorSpec>();
    for (const connector of catalog) {
      if (connector.kind !== kind) continue;
      const family = getConnectorFamily(connector, catalog);
      families.set(family.name, family);
    }
    return shuffleConnectors([...families.values()]);
  }, [data?.connectors, kind]);
};

export default EmptyGraphic;
