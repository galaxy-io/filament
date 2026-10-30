import type { FC } from "react";

import { css } from "@linaria/core";
import { ListIcon } from "@phosphor-icons/react";

import Widget from "@galaxy-io/dls/widget/Widget";

import { OBSERVABILITY_RUNS_VIEW_TO_TABLE_HEADER_MAP } from "@/pages/observability/components/runs/constants";
import ObservabilityRunsPastTable from "@/pages/observability/components/runs/ObservabilityRunsPastTable";
import ObservabilityRunsUpcomingTable from "@/pages/observability/components/runs/ObservabilityRunsUpcomingTable";
import { ObservabilityRunsView } from "@/pages/observability/types";

import { useObservabilitySearch } from "@/module/hooks";

const OBSERVABILITY_RUNS_TABLE_WIDGET_CSS = css`
  flex: 1 1 0;
  min-height: 0;
  border-inline: none;
  border-bottom: none;
  border-radius: 0;
`;

const ObservabilityRunsTableWidget: FC = () => {
  const { runs: view } = useObservabilitySearch();

  return (
    <Widget
      className={OBSERVABILITY_RUNS_TABLE_WIDGET_CSS}
      isFlush
      gap={0}
      header={OBSERVABILITY_RUNS_VIEW_TO_TABLE_HEADER_MAP[view]}
      icon={ListIcon}
    >
      {view === ObservabilityRunsView.PAST ? (
        <ObservabilityRunsPastTable />
      ) : (
        <ObservabilityRunsUpcomingTable />
      )}
    </Widget>
  );
};

export default ObservabilityRunsTableWidget;
