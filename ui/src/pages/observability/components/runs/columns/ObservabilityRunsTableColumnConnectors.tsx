import { useMemo } from "react";

import type { RunInfo } from "@/gen/ingestion/v1/runs_pb";

import { OBSERVABILITY_CONNECTIONS_INPUT } from "@/pages/observability/constants";
import PipelineFlow, { PipelineFlowSize } from "@/pages/pipelines/components/flow/PipelineFlow";
import { mapConnectionIdToFlowConnection } from "@/pages/pipelines/components/flow/utils";

import { useListConnectionsQuery } from "@/api/queries/connections";

interface ObservabilityRunsTableColumnConnectorsProps {
  runInfo: RunInfo;
}

const ObservabilityRunsTableColumnConnectors = ({
  runInfo,
}: ObservabilityRunsTableColumnConnectorsProps) => {
  const { data } = useListConnectionsQuery({
    input: OBSERVABILITY_CONNECTIONS_INPUT,
  });

  const connectionsById = useMemo(
    () => new Map((data?.connections ?? []).map((connection) => [connection.id, connection])),
    [data?.connections],
  );

  return (
    <PipelineFlow
      source={mapConnectionIdToFlowConnection(runInfo.sourceConnectionId, connectionsById)}
      sinks={
        runInfo.sinkConnectionId
          ? [mapConnectionIdToFlowConnection(runInfo.sinkConnectionId, connectionsById)]
          : []
      }
      size={PipelineFlowSize.SMALL}
    />
  );
};

export default ObservabilityRunsTableColumnConnectors;
