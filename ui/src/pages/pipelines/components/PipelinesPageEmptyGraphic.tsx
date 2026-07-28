import type { ReactNode } from "react";

import { styled } from "@linaria/react";
import { FlowArrowIcon } from "@phosphor-icons/react";

import FlexItem from "@galaxy-io/dls/containers/FlexItem";
import FlexWrapper, {
  AlignItems,
  FlexDirection,
  JustifyContent,
} from "@galaxy-io/dls/containers/FlexWrapper";
import Icon, { IconVariant, IconWeight } from "@galaxy-io/dls/icons/Icon";
import Text, { TextSize, TextVariant, TextWeight } from "@galaxy-io/dls/text/Text";
import { withTheme } from "@galaxy-io/dls/theme/GalaxyTheme";
import type { PropsWithTheme } from "@galaxy-io/dls/theme/types";

import { ConnectorKind } from "@/gen/ingestion/v1/common_pb";

import EmptyGraphic, {
  EmptyGraphicGhostBar,
  EmptyGraphicGhostTileFallback,
} from "@/components/EmptyGraphic";

import ConnectorTile, { ConnectorTileSize } from "@/pages/connectors/components/ConnectorTile";

import { useListConnectorsQuery } from "@/api/queries/connectors";

interface PipelinesEmptyRow {
  sinkCount: number;
  nameWidth: number;
  metaWidth: number;
}

const PIPELINES_EMPTY_ROWS: PipelinesEmptyRow[] = [
  { sinkCount: 2, nameWidth: 120, metaWidth: 60 },
  { sinkCount: 1, nameWidth: 90, metaWidth: 44 },
  { sinkCount: 3, nameWidth: 140, metaWidth: 70 },
];

const PIPELINES_EMPTY_SINK_OFFSETS = [0, 1, 2];

const VeilWrapper = styled.div`
  width: 100%;

  display: flex;
  flex-direction: column;
  gap: 8px;

  mask-image: linear-gradient(180deg, black 55%, transparent 110%);
  -webkit-mask-image: linear-gradient(180deg, black 55%, transparent 110%);
`;

const GhostRow = withTheme(styled.div<PropsWithTheme>`
  height: 44px;

  display: flex;
  align-items: center;
  gap: 12px;
  padding: 0 12px;

  background-color: ${({ theme }) => theme.color.background.primary};

  border: 0.5px solid ${({ theme }) => theme.color.border.primary};
  border-radius: 5px;
`);

const GhostFlowWrapper = styled.div`
  display: flex;
  align-items: center;
  gap: 8px;

  opacity: 0.35;
`;

const GhostSinkCluster = styled.div`
  display: flex;
  align-items: center;
  gap: 4px;
`;

interface PipelinesPageEmptyGraphicProps {
  actions?: ReactNode;
}

const PipelinesPageEmptyGraphic = ({ actions }: PipelinesPageEmptyGraphicProps) => {
  const { data } = useListConnectorsQuery();
  const connectors = data?.connectors ?? [];
  const sourceSpecs = connectors.filter((connector) => connector.kind === ConnectorKind.SOURCE);
  const sinkSpecs = connectors.filter((connector) => connector.kind === ConnectorKind.SINK);

  return (
    <FlexWrapper
      fillWidth
      fillHeight
      direction={FlexDirection.COLUMN}
      alignItems={AlignItems.CENTER}
      justifyContent={JustifyContent.CENTER}
      gap={20}
    >
      <EmptyGraphic>
        <VeilWrapper>
          {PIPELINES_EMPTY_ROWS.map((row, rowIndex) => {
            const sourceSpec = sourceSpecs.length
              ? sourceSpecs[rowIndex % sourceSpecs.length]
              : undefined;

            return (
              <GhostRow key={row.nameWidth}>
                <GhostFlowWrapper>
                  {sourceSpec ? (
                    <ConnectorTile
                      connector={sourceSpec.name}
                      spec={sourceSpec}
                      size={ConnectorTileSize.SMALL}
                    />
                  ) : (
                    <EmptyGraphicGhostTileFallback />
                  )}
                  <Icon
                    component={FlowArrowIcon}
                    size={16}
                    variant={IconVariant.PRIMARY}
                    weight={IconWeight.REGULAR}
                  />
                  <GhostSinkCluster>
                    {PIPELINES_EMPTY_SINK_OFFSETS.slice(0, row.sinkCount).map((offset) => {
                      const sinkSpec = sinkSpecs.length
                        ? sinkSpecs[(rowIndex + offset) % sinkSpecs.length]
                        : undefined;

                      return sinkSpec ? (
                        <ConnectorTile
                          key={offset}
                          connector={sinkSpec.name}
                          spec={sinkSpec}
                          size={ConnectorTileSize.SMALL}
                        />
                      ) : (
                        <EmptyGraphicGhostTileFallback key={offset} />
                      );
                    })}
                  </GhostSinkCluster>
                </GhostFlowWrapper>
                <EmptyGraphicGhostBar $width={row.nameWidth} />
                <FlexItem grow={1} />
                <EmptyGraphicGhostBar $width={row.metaWidth} />
              </GhostRow>
            );
          })}
        </VeilWrapper>
      </EmptyGraphic>
      <FlexWrapper direction={FlexDirection.COLUMN} alignItems={AlignItems.CENTER} gap={8}>
        <Text size={TextSize.BODY_LG} weight={TextWeight.MEDIUM}>
          No pipelines found
        </Text>
        <Text size={TextSize.BODY_MD} variant={TextVariant.SECONDARY}>
          Create pipelines to move data between your connectors.
        </Text>
      </FlexWrapper>
      {actions}
    </FlexWrapper>
  );
};

export default PipelinesPageEmptyGraphic;
