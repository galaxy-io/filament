import { createFileRoute, Outlet, useParams } from "@tanstack/react-router";

import PipelineLayout from "@/layouts/pipeline/PipelineLayout";
import { PipelineStatus } from "@/layouts/pipeline/types";

const PipelineLayoutRoute = () => {
  const { id } = useParams({ from: "/pipelines/$id" });

  // TODO: Fetch pipeline data from API
  const pipelineName = "Untitled Pipeline";
  const pipelineStatus = PipelineStatus.DRAFT;
  const pipelineSource = "postgres";
  const pipelineSinks = ["postgres", "s3", "stdout"];

  return (
    <PipelineLayout
      pipelineId={id}
      name={pipelineName}
      status={pipelineStatus}
      source={pipelineSource}
      sinks={pipelineSinks}
    >
      <Outlet />
    </PipelineLayout>
  );
};

export const Route = createFileRoute("/pipelines/$id")({
  component: PipelineLayoutRoute,
});
