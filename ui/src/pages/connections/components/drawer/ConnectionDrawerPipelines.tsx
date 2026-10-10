import { type FC, useMemo } from "react";

import { FlowArrowIcon } from "@phosphor-icons/react";

import Flex, { AlignItems, FlexDirection } from "@galaxy-io/dls/layout/Flex";

import PipelineCard from "@/components/pipelines/PipelineCard";

import ConnectionDrawerSection from "@/pages/connections/components/drawer/ConnectionDrawerSection";
import { usePipelineConnectionMap } from "@/pages/connections/hooks/usePipelineConnectionMap";

interface ConnectionDrawerPipelinesProps {
  connectionId: string;
}

const ConnectionDrawerPipelines: FC<ConnectionDrawerPipelinesProps> = ({ connectionId }) => {
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
      isFlush
    >
      <Flex alignItems={AlignItems.STRETCH} fillWidth direction={FlexDirection.COLUMN}>
        {connectedPipelines.map((pipeline) => (
          <PipelineCard key={pipeline.id} pipeline={pipeline} />
        ))}
      </Flex>
    </ConnectionDrawerSection>
  );
};

export default ConnectionDrawerPipelines;
