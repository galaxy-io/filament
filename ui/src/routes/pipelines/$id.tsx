import { create } from "@bufbuild/protobuf";
import { Code, ConnectError } from "@connectrpc/connect";
import { ArrowLeftIcon, ImageBrokenIcon } from "@phosphor-icons/react";
import { createFileRoute, notFound, useNavigate } from "@tanstack/react-router";

import Button from "@galaxy-io/dls/buttons/Button";
import Icon, { IconVariant } from "@galaxy-io/dls/icons/Icon";

import { GetPipelineRequestSchema } from "@/gen/ingestion/v1/pipelines_pb";

import ErrorLayout from "@/layouts/ErrorLayout";

import PipelinePage from "@/pages/pipelines/PipelinePage";

import { createListConnectionsQueryOptions } from "@/api/queries/connections";
import { createGetPipelineQueryOptions } from "@/api/queries/pipelines";
import { queryClient } from "@/api/queryClient";
import { transport } from "@/api/transport";

const PipelineNotFoundComponent = () => {
  const navigate = useNavigate();

  const handleGoToPipelines = () => {
    void navigate({ to: "/pipelines" });
  };

  return (
    <ErrorLayout
      icon={
        <Icon
          component={ImageBrokenIcon}
          size={24}
          variant={IconVariant.SECONDARY}
        />
      }
      header="Pipeline not found"
      message="The pipeline you are looking for does not exist"
      actions={
        <Button
          label="Go back to pipelines"
          icon={ArrowLeftIcon}
          onClick={handleGoToPipelines}
        />
      }
    />
  );
};

export const Route = createFileRoute("/pipelines/$id")({
  loader: async ({ params }) => {
    try {
      await Promise.all([
        queryClient.ensureQueryData(
          createGetPipelineQueryOptions({
            input: create(GetPipelineRequestSchema, { id: params.id }),
            transport,
          }),
        ),
        queryClient.ensureQueryData(
          createListConnectionsQueryOptions({ transport }),
        ),
      ]);
    } catch (error) {
      if (error instanceof ConnectError && error.code === Code.NotFound) {
        throw notFound();
      }
      throw error;
    }
  },
  notFoundComponent: PipelineNotFoundComponent,
  component: PipelinePage,
});
