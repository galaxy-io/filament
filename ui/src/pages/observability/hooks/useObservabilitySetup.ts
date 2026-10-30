import { useMemo } from "react";

import {
  OBSERVABILITY_SETUP_PIPELINES_INPUT,
  OBSERVABILITY_SETUP_SINKS_INPUT,
  OBSERVABILITY_SETUP_SOURCES_INPUT,
  OBSERVABILITY_SETUP_STEP_ORDER,
} from "@/pages/observability/components/setup/constants";
import { ObservabilitySetupStep } from "@/pages/observability/components/setup/types";

import { useSuspenseListConnectionsQuery } from "@/api/queries/connections";
import { useSuspenseListPipelinesQuery } from "@/api/queries/pipelines";

interface ObservabilitySetup {
  completedSteps: Set<ObservabilitySetupStep>;
  activeStep: ObservabilitySetupStep | null;
  completedCount: number;
  isComplete: boolean;
}

export const useObservabilitySetup = (): ObservabilitySetup => {
  const { data: sourcesData } = useSuspenseListConnectionsQuery({
    input: OBSERVABILITY_SETUP_SOURCES_INPUT,
  });
  const { data: sinksData } = useSuspenseListConnectionsQuery({
    input: OBSERVABILITY_SETUP_SINKS_INPUT,
  });
  const { data: pipelinesData } = useSuspenseListPipelinesQuery({
    input: OBSERVABILITY_SETUP_PIPELINES_INPUT,
  });

  const hasSource = sourcesData.connections.length > 0;
  const hasSink = sinksData.connections.length > 0;
  const hasPipeline = pipelinesData.pipelines.length > 0;

  return useMemo(() => {
    const completedSteps = new Set<ObservabilitySetupStep>();

    if (hasSource) {
      completedSteps.add(ObservabilitySetupStep.SOURCE);
    }
    if (hasSink) {
      completedSteps.add(ObservabilitySetupStep.SINK);
    }
    if (hasPipeline) {
      completedSteps.add(ObservabilitySetupStep.PIPELINE);
    }

    const activeStep =
      OBSERVABILITY_SETUP_STEP_ORDER.find((step) => !completedSteps.has(step)) ?? null;

    return {
      completedSteps,
      activeStep,
      completedCount: completedSteps.size,
      isComplete: activeStep === null,
    };
  }, [hasSource, hasSink, hasPipeline]);
};
