import type { FC } from "react";

import { match } from "ts-pattern";

import ObservabilityBandRunsPastTile from "@/pages/observability/components/band/ObservabilityBandRunsPastTile";
import ObservabilityBandRunsUpcomingTile from "@/pages/observability/components/band/ObservabilityBandRunsUpcomingTile";
import { ObservabilityRunsView } from "@/pages/observability/types";

import { useObservabilitySearch } from "@/module/hooks";

const ObservabilityBandRunsTile: FC = () => {
  const { runs: view } = useObservabilitySearch();

  return match(view)
    .with(ObservabilityRunsView.PAST, () => <ObservabilityBandRunsPastTile />)
    .with(ObservabilityRunsView.UPCOMING, () => <ObservabilityBandRunsUpcomingTile />)
    .exhaustive();
};

export default ObservabilityBandRunsTile;
