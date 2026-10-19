import type { FC } from "react";

import { css } from "@linaria/core";
import { CpuIcon, GaugeIcon } from "@phosphor-icons/react";

import ChartGroupProvider from "@galaxy-io/dls/charts/ChartGroupProvider";
import Flex, { AlignItems, FlexDirection, FlexWrap } from "@galaxy-io/dls/layout/Flex";
import FlexItem from "@galaxy-io/dls/layout/FlexItem";
import PageLayout from "@galaxy-io/dls/layout/PageLayout";
import ScrollArea from "@galaxy-io/dls/layout/ScrollArea";
import { HAIRLINE_WIDTH } from "@galaxy-io/dls/styles/mixins";

import { MetricDimension } from "@/gen/metrics/v1/metrics_pb";

import ObservabilityMetricsWidget from "@/pages/observability/components/metrics/ObservabilityMetricsWidget";
import ObservabilityPageActions from "@/pages/observability/components/ObservabilityPageActions";
import ObservabilityRunsChartWidget from "@/pages/observability/components/runs/ObservabilityRunsChartWidget";
import ObservabilityRunsTableWidget from "@/pages/observability/components/runs/ObservabilityRunsTableWidget";
import ObservabilitySetupChecklist from "@/pages/observability/components/setup/ObservabilitySetupChecklist";
import {
  OBSERVABILITY_THROUGHPUT_VIEW_TO_CONFIG_MAP,
  OBSERVABILITY_THROUGHPUT_VIEWS,
  OBSERVABILITY_USAGE_VIEW_TO_CONFIG_MAP,
  OBSERVABILITY_USAGE_VIEWS,
} from "@/pages/observability/components/timeseries/constants";
import ObservabilityTimeseriesWidget from "@/pages/observability/components/timeseries/ObservabilityTimeseriesWidget";
import { OBSERVABILITY_WIDGET_BASIS } from "@/pages/observability/constants";
import { useObservabilityScrollTakeover } from "@/pages/observability/hooks/useObservabilityScrollTakeover";
import { useObservabilitySetup } from "@/pages/observability/hooks/useObservabilitySetup";
import { ObservabilityThroughputView, ObservabilityUsageView } from "@/pages/observability/types";

const OBSERVABILITY_PAGE_SCROLL_CSS = css`
  container-type: size;
`;

const OBSERVABILITY_RUNS_TAKEOVER_HEIGHT = `calc(100cqh + ${HAIRLINE_WIDTH})`;

const ObservabilityPage: FC = () => {
  const { isComplete } = useObservabilitySetup();
  const { scrollRef, takeoverRef } = useObservabilityScrollTakeover(isComplete);

  if (!isComplete) {
    return (
      <PageLayout header="Observability">
        <ObservabilitySetupChecklist />
      </PageLayout>
    );
  }

  return (
    <PageLayout header="Observability" actions={<ObservabilityPageActions />}>
      <ChartGroupProvider shouldShareTooltip>
        <ScrollArea ref={scrollRef} className={OBSERVABILITY_PAGE_SCROLL_CSS}>
          <Flex
            alignItems={AlignItems.START}
            direction={FlexDirection.COLUMN}
            padding={16}
            gap={12}
            fillWidth
          >
            <ObservabilityMetricsWidget />
            <Flex gap={12} alignItems={AlignItems.STRETCH} wrap={FlexWrap.WRAP} fillWidth>
              <FlexItem grow={1} basis={OBSERVABILITY_WIDGET_BASIS} minWidth={0}>
                <ObservabilityTimeseriesWidget
                  header="Throughput"
                  icon={GaugeIcon}
                  isStacked
                  views={OBSERVABILITY_THROUGHPUT_VIEWS}
                  viewToConfigMap={OBSERVABILITY_THROUGHPUT_VIEW_TO_CONFIG_MAP}
                  defaultView={ObservabilityThroughputView.RECORDS}
                  viewSearchKey="throughput"
                  pivotSearchKey="throughputPivot"
                />
              </FlexItem>
              <FlexItem grow={1} basis={OBSERVABILITY_WIDGET_BASIS} minWidth={0}>
                <ObservabilityTimeseriesWidget
                  header="Usage"
                  icon={CpuIcon}
                  views={OBSERVABILITY_USAGE_VIEWS}
                  viewToConfigMap={OBSERVABILITY_USAGE_VIEW_TO_CONFIG_MAP}
                  defaultView={ObservabilityUsageView.CPU}
                  defaultPivot={MetricDimension.PIPELINE_ID}
                  viewSearchKey="usage"
                  pivotSearchKey="usagePivot"
                />
              </FlexItem>
            </Flex>
            <FlexItem fillWidth>
              <ObservabilityRunsChartWidget />
            </FlexItem>
          </Flex>
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
