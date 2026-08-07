import { useMemo } from "react";

import { ConnectorKind } from "@/gen/ingestion/v1/common_pb";

import { OBSERVABILITY_SETUP_STEP_ORDER } from "@/pages/observability/components/setup/constants";
import { ObservabilitySetupStep } from "@/pages/observability/components/setup/types";
import {
  OBSERVABILITY_CONNECTIONS_INPUT,
  OBSERVABILITY_PIPELINES_INPUT,
} from "@/pages/observability/constants";

import { useSuspenseListConnectionsQuery } from "@/api/queries/connections";
import { useSuspenseListPipelinesQuery } from "@/api/queries/pipelines";

export interface ObservabilitySetup {
  completedSteps: Set<ObservabilitySetupStep>;
  activeStep: ObservabilitySetupStep | null;
  completedCount: number;
  isComplete: boolean;
}

export const useObservabilitySetup = (): ObservabilitySetup => {
  const { data: connectionsData } = useSuspenseListConnectionsQuery({
    input: OBSERVABILITY_CONNECTIONS_INPUT,
  });
  const { data: pipelinesData } = useSuspenseListPipelinesQuery({
    input: OBSERVABILITY_PIPELINES_INPUT,
  });

  return useMemo(() => {
    const completedSteps = new Set<ObservabilitySetupStep>();

    if (connectionsData.connections.some(({ kind }) => kind === ConnectorKind.SOURCE)) {
      completedSteps.add(ObservabilitySetupStep.SOURCE);
    }
    if (connectionsData.connections.some(({ kind }) => kind === ConnectorKind.SINK)) {
      completedSteps.add(ObservabilitySetupStep.SINK);
    }
    if (pipelinesData.pipelines.length) {
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
  }, [connectionsData.connections, pipelinesData.pipelines]);
};
