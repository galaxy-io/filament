import type { FC } from "react";

import { css } from "@linaria/core";
import { styled } from "@linaria/react";
import { PlusIcon } from "@phosphor-icons/react";

import GalaxyFilamentWordmark from "@galaxy-io/dls/brand/GalaxyFilamentWordmark";
import Button, { ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import ChartGroupProvider from "@galaxy-io/dls/charts/ChartGroupProvider";
import Box from "@galaxy-io/dls/layout/Box";
import Divider from "@galaxy-io/dls/layout/Divider";
import Flex, { FlexDirection } from "@galaxy-io/dls/layout/Flex";
import PageLayout from "@galaxy-io/dls/layout/PageLayout";
import ScrollArea from "@galaxy-io/dls/layout/ScrollArea";
import { HAIRLINE_WIDTH } from "@galaxy-io/dls/styles/mixins";
import { t } from "@galaxy-io/dls/theme/tokens/t";

import { ConnectorKind } from "@/gen/ingestion/v1/common_pb";

import { PIPELINE_CREATE_TITLE } from "@/components/pipelines/constants";

import ObservabilityBand from "@/pages/observability/components/band/ObservabilityBand";
import ObservabilityConnectionsWidget from "@/pages/observability/components/connections/ObservabilityConnectionsWidget";
import ObservabilityFailuresBanner from "@/pages/observability/components/failures/ObservabilityFailuresBanner";
import ObservabilityPageActions from "@/pages/observability/components/ObservabilityPageActions";
import ObservabilityPipelinesWidget from "@/pages/observability/components/pipelines/ObservabilityPipelinesWidget";
import ObservabilityRunsTableWidget from "@/pages/observability/components/runs/ObservabilityRunsTableWidget";
import ObservabilityScheduledWidget from "@/pages/observability/components/scheduled/ObservabilityScheduledWidget";
import ObservabilitySetupChecklist from "@/pages/observability/components/setup/ObservabilitySetupChecklist";
import {
  OBSERVABILITY_BLOCK_CONNECTIONS_HEIGHT,
  OBSERVABILITY_BLOCK_MIN_HEIGHT,
  OBSERVABILITY_BLOCK_SIDE_STACKED_HEIGHT,
  OBSERVABILITY_BLOCK_SIDE_WIDTH,
  OBSERVABILITY_BLOCK_STACK_MAX_WIDTH,
  OBSERVABILITY_LAYOUT_CONTAINER_NAME,
  OBSERVABILITY_WORDMARK_SIZE,
} from "@/pages/observability/constants";
import { useObservabilityScrollTakeover } from "@/pages/observability/hooks/useObservabilityScrollTakeover";
import { useObservabilitySetup } from "@/pages/observability/hooks/useObservabilitySetup";

import { useFilamentFlowOpen } from "@/module/hooks";
import { Flow } from "@/module/types";

const OBSERVABILITY_PAGE_SCROLL_CSS = css`
  container-type: size;
`;

const OBSERVABILITY_RUNS_TAKEOVER_HEIGHT = `calc(100cqh + ${HAIRLINE_WIDTH})`;

const ObservabilityLayout = styled.div`
  container: ${OBSERVABILITY_LAYOUT_CONTAINER_NAME} / inline-size;
  width: 100%;
`;

const ObservabilityBlockSide = styled.div`
  contain: size;
  display: grid;
  grid-template-rows: minmax(0, 1fr) ${OBSERVABILITY_BLOCK_CONNECTIONS_HEIGHT}px ${OBSERVABILITY_BLOCK_CONNECTIONS_HEIGHT}px;
  gap: ${t.space[12]};

  & > * {
    min-height: 0;
  }
`;

const ObservabilityBlock = styled.div`
  display: grid;
  grid-template-columns: minmax(0, 1fr) ${OBSERVABILITY_BLOCK_SIDE_WIDTH}px;
  grid-template-rows: minmax(${OBSERVABILITY_BLOCK_MIN_HEIGHT}px, auto);
  gap: ${t.space[12]};

  @container ${OBSERVABILITY_LAYOUT_CONTAINER_NAME} (max-width: ${OBSERVABILITY_BLOCK_STACK_MAX_WIDTH}px) {
    grid-template-columns: minmax(0, 1fr);
    grid-template-rows: auto ${OBSERVABILITY_BLOCK_SIDE_STACKED_HEIGHT}px;

    ${ObservabilityBlockSide} {
      grid-template-rows: minmax(0, 1fr);
      grid-template-columns: repeat(3, minmax(0, 1fr));
    }
  }
`;

const ObservabilityPage: FC = () => {
  const { isComplete } = useObservabilitySetup();
  const { scrollRef, takeoverRef } = useObservabilityScrollTakeover(isComplete);
  const openFlow = useFilamentFlowOpen();

  const handleNewPipeline = () => {
    openFlow(Flow.CREATE_PIPELINE);
  };

  if (!isComplete) {
    return <ObservabilitySetupChecklist />;
  }

  return (
    <PageLayout
      header={<GalaxyFilamentWordmark size={OBSERVABILITY_WORDMARK_SIZE} ariaLabel="Filament" />}
      actions={
        <>
          <ObservabilityPageActions />
          <Button
            label={PIPELINE_CREATE_TITLE}
            icon={PlusIcon}
            variant={ButtonVariant.PRIMARY}
            onClick={handleNewPipeline}
          />
        </>
      }
      banner={<ObservabilityFailuresBanner />}
    >
      <ChartGroupProvider shouldShareTooltip>
        <ScrollArea ref={scrollRef} className={OBSERVABILITY_PAGE_SCROLL_CSS}>
          <ObservabilityLayout>
            <ObservabilityBand />
            <Divider />
            <Box padding={16} fillWidth>
              <ObservabilityBlock>
                <ObservabilityPipelinesWidget />
                <ObservabilityBlockSide>
                  <ObservabilityScheduledWidget />
                  <ObservabilityConnectionsWidget kind={ConnectorKind.SOURCE} />
                  <ObservabilityConnectionsWidget kind={ConnectorKind.SINK} />
                </ObservabilityBlockSide>
              </ObservabilityBlock>
            </Box>
          </ObservabilityLayout>
          <Flex
            ref={takeoverRef}
            direction={FlexDirection.COLUMN}
            height={OBSERVABILITY_RUNS_TAKEOVER_HEIGHT}
          >
            <ObservabilityRunsTableWidget />
          </Flex>
        </ScrollArea>
      </ChartGroupProvider>
    </PageLayout>
  );
};

export default ObservabilityPage;
