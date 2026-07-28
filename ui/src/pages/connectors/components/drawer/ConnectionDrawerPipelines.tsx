import { useMemo } from "react";

import { FlowArrowIcon } from "@phosphor-icons/react";

import FlexWrapper, { FlexDirection } from "@galaxy-io/dls/containers/FlexWrapper";

import type { Connection } from "@/gen/ingestion/v1/connections_pb";

import ConnectionDrawerSection from "@/pages/connectors/components/drawer/ConnectionDrawerSection";
import { usePipelineConnectionMap } from "@/pages/connectors/hooks/usePipelineConnectionMap";
import PipelineCard from "@/pages/pipelines/components/card/PipelineCard";

interface ConnectionDrawerPipelinesProps {
  connectionId: Connection["id"];
}

const ConnectionDrawerPipelines = ({ connectionId }: ConnectionDrawerPipelinesProps) => {
  const { pipelines, connectionIdsByPipelineId } = usePipelineConnectionMap();

  const connectedPipelines = useMemo(
    () =>
      pipelines.filter((pipeline) => connectionIdsByPipelineId.get(pipeline.id)?.has(connectionId)),
    [pipelines, connectionIdsByPipelineId, connectionId],
  );

  return (
    <ConnectionDrawerSection
      header="Pipelines"
      icon={FlowArrowIcon}
      count={connectedPipelines.length}
      emptyHeader="No pipelines"
      emptyMessage="This connection is not used in any pipelines."
      isOpenInitial
    >
      <FlexWrapper fillWidth direction={FlexDirection.COLUMN}>
        {connectedPipelines.map((pipeline) => (
          <PipelineCard key={pipeline.id} pipeline={pipeline} />
        ))}
      </FlexWrapper>
    </ConnectionDrawerSection>
  );
};

export default ConnectionDrawerPipelines;
