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

  if (!data.pipeline) {
    throw new Error(`Pipeline ${id} not found`);
  }

  return <PipelineSettingsForm key={data.pipeline.id} pipeline={data.pipeline} />;
};

export default PipelineSettingsPage;
