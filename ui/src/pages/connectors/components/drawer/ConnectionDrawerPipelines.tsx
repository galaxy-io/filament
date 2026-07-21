import { useMemo } from "react";

import { FlowArrowIcon } from "@phosphor-icons/react";

import Accordion from "@galaxy-io/dls/accordion/Accordion";
import Badge, { BadgeSize, BadgeVariant } from "@galaxy-io/dls/badge/Badge";
import FlexWrapper, { FlexDirection } from "@galaxy-io/dls/containers/FlexWrapper";
import Icon, { IconVariant } from "@galaxy-io/dls/icons/Icon";

import EmptyLayout, { EmptyLayoutSize } from "@/layouts/EmptyLayout";

import PipelineCard from "@/pages/pipelines/components/PipelineCard";

import { useListPipelinesQuery } from "@/api/queries/pipelines";

interface ConnectionDrawerPipelinesProps {
  connectionId: string;
}

const ConnectionDrawerPipelines = ({ connectionId }: ConnectionDrawerPipelinesProps) => {
  const { data: pipelinesData } = useListPipelinesQuery();

  const connectedPipelines = useMemo(() => {
    if (!pipelinesData?.pipelines) return [];

    return pipelinesData.pipelines.filter((pipeline) =>
      pipeline.nodes.some((node) => node.connectionId === connectionId),
    );
  }, [pipelinesData?.pipelines, connectionId]);

  return (
    <Accordion
      header="Pipelines"
      icon={FlowArrowIcon}
      metric={
        <Badge
          count={connectedPipelines.length}
          size={BadgeSize.SMALL}
          variant={BadgeVariant.SECONDARY}
        />
      }
      isOpenInitial
      padding={connectedPipelines.length > 0 ? 0 : undefined}
    >
      {connectedPipelines.length === 0 ? (
        <EmptyLayout
          size={EmptyLayoutSize.SMALL}
          icon={<Icon component={FlowArrowIcon} size={16} variant={IconVariant.TERTIARY} />}
          header="No pipelines"
          message="This connection is not used in any pipelines."
        />
      ) : (
        <FlexWrapper fillWidth direction={FlexDirection.COLUMN}>
          {connectedPipelines.map((pipeline) => (
            <PipelineCard key={pipeline.id} pipeline={pipeline} isCompact />
          ))}
        </FlexWrapper>
      )}
    </Accordion>
  );
};

export default ConnectionDrawerPipelines;
