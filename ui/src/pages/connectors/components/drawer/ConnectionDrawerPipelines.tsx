import { useMemo } from "react";

import { create } from "@bufbuild/protobuf";
import { createQueryOptions, useTransport } from "@connectrpc/connect-query";
import { FlowArrowIcon } from "@phosphor-icons/react";
import { useQueries } from "@tanstack/react-query";

import Accordion from "@galaxy-io/dls/accordion/Accordion";
import Badge, { BadgeSize, BadgeVariant } from "@galaxy-io/dls/badge/Badge";
import FlexWrapper, { FlexDirection } from "@galaxy-io/dls/containers/FlexWrapper";
import Icon, { IconVariant } from "@galaxy-io/dls/icons/Icon";

import EmptyLayout, { EmptyLayoutSize } from "@/layouts/EmptyLayout";

import PipelineCard from "@/pages/pipelines/components/PipelineCard";

import { useListPipelinesQuery } from "@/api/queries/pipelines";

import { GetPipelineVersionRequestSchema } from "@/gen/ingestion/v1/pipelines_pb";
import { IngestionService } from "@/gen/ingestion/v1/service_pb";

interface ConnectionDrawerPipelinesProps {
  connectionId: string;
}

const ConnectionDrawerPipelines = ({ connectionId }: ConnectionDrawerPipelinesProps) => {
  const transport = useTransport();
  const { data: pipelinesData } = useListPipelinesQuery();

  const pipelines = pipelinesData?.pipelines;

  // The graph lives on each pipeline's current version; NotFound (no saved
  // version yet) means the pipeline can't reference any connection
  const versionResults = useQueries({
    queries: (pipelines ?? []).map((pipeline) => ({
      ...createQueryOptions(
        IngestionService.method.getPipelineVersion,
        create(GetPipelineVersionRequestSchema, { pipelineId: pipeline.id }),
        { transport },
      ),
      retry: false,
    })),
  });

  const connectedPipelines = useMemo(() => {
    if (!pipelines) return [];

    return pipelines.filter((_, index) =>
      versionResults[index]?.data?.version?.nodes.some(
        (node) => node.connectionId === connectionId,
      ),
    );
  }, [pipelines, versionResults, connectionId]);

  return (
    <Accordion
      header="Pipelines"
      metric={
        <Badge
          count={connectedPipelines.length}
          size={BadgeSize.SMALL}
          variant={BadgeVariant.SECONDARY}
        />
      }
      isOpenInitial
      padding={connectedPipelines.length > 0 ? 0 : "24px"}
    >
      {connectedPipelines.length === 0 ? (
        <EmptyLayout
          size={EmptyLayoutSize.SMALL}
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
