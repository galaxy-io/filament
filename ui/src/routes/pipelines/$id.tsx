import { create } from "@bufbuild/protobuf";
import { createFileRoute, Outlet, useParams } from "@tanstack/react-router";

import PipelineLayout from "@/layouts/pipeline/PipelineLayout";
import { PipelineStatus } from "@/layouts/pipeline/types";

import { useGetPipelineQuery } from "@/api/queries/pipelines";

import { ConnectorKind } from "@/gen/ingestion/v1/common_pb";
import { GetPipelineRequestSchema } from "@/gen/ingestion/v1/pipelines_pb";

const PipelineLayoutRoute = () => {
  const { id } = useParams({ from: "/pipelines/$id" });
  const { data, isLoading } = useGetPipelineQuery({
    input: create(GetPipelineRequestSchema, { id }),
  });

  const pipeline = data?.pipeline;
  const sources = pipeline?.nodes.filter((n) => n.kind === ConnectorKind.SOURCE) ?? [];
  const sinks = pipeline?.nodes.filter((n) => n.kind === ConnectorKind.SINK) ?? [];

  if (isLoading) {
    return null;
  }

  return (
    <PipelineLayout
      pipelineId={id}
      name={pipeline?.name || "Untitled Pipeline"}
      status={PipelineStatus.DRAFT}
      source={sources[0]?.connectionId ?? ""}
      sinks={sinks.map((s) => s.connectionId)}
    >
      <Outlet />
    </PipelineLayout>
  );
};

export const Route = createFileRoute("/pipelines/$id")({
  component: PipelineLayoutRoute,
});
