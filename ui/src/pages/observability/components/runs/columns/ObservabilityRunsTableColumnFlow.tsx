import { type FC, useMemo } from "react";

import type { RunInfo } from "@/gen/ingestion/v1/runs_pb";

import PipelineFlow, { PipelineFlowSize } from "@/components/pipelines/PipelineFlow";
import { mapConnectionIdToFlowConnection } from "@/components/pipelines/utils";

import { OBSERVABILITY_CONNECTIONS_INPUT } from "@/pages/observability/constants";

import { useListConnectionsQuery } from "@/api/queries/connections";

interface ObservabilityRunsTableColumnFlowProps {
  runInfo: RunInfo;
}

const ObservabilityRunsTableColumnFlow: FC<ObservabilityRunsTableColumnFlowProps> = ({
  runInfo,
}) => {
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

export default ObservabilityRunsTableColumnFlow;
