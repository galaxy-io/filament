import { create } from "@bufbuild/protobuf";
import { useParams } from "@tanstack/react-router";

import { GetPipelineRequestSchema } from "@/gen/ingestion/v1/pipelines_pb";

import PipelineSettingsForm from "@/pages/pipelines/settings/PipelineSettingsForm";

import { useSuspenseGetPipelineQuery } from "@/api/queries/pipelines";

const PipelineSettingsPage = () => {
  const { id } = useParams({ from: "/pipelines/$id" });

  const { data } = useSuspenseGetPipelineQuery({
    input: create(GetPipelineRequestSchema, { id }),
  });
  const pipeline = data.pipeline;

  if (!pipeline) {
    throw new Error(`Pipeline ${id} not found`);
  }

  return <PipelineSettingsForm key={pipeline.id} pipeline={pipeline} />;
};

export default PipelineSettingsPage;
