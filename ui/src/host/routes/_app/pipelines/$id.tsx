import type { FC } from "react";

import { Code, ConnectError } from "@connectrpc/connect";
import { ArrowLeftIcon, ImageBrokenIcon } from "@phosphor-icons/react";
import { createFileRoute, type ErrorComponentProps, useNavigate } from "@tanstack/react-router";

import Button from "@galaxy-io/dls/buttons/Button";
import ErrorLayout from "@galaxy-io/dls/layout/ErrorLayout";

import PipelinePage from "@/pages/pipelines/PipelinePage";

import { pipelineSearchSchema } from "@/module/schemas";

import { DefaultErrorComponent } from "@/host/router";

const PipelineNotFoundComponent: FC = () => {
  const navigate = useNavigate();

  const handleGoToPipelines = () => {
    void navigate({ to: "/pipelines" });
  };

  return (
    <ErrorLayout
      icon={ImageBrokenIcon}
      header="Pipeline not found"
      description="The pipeline you are looking for does not exist"
      actions={
        <Button label="Go back to pipelines" icon={ArrowLeftIcon} onClick={handleGoToPipelines} />
      }
    />
  );
};

const PipelineErrorComponent: FC<ErrorComponentProps> = ({ error }) =>
  error instanceof ConnectError && error.code === Code.NotFound ? (
    <PipelineNotFoundComponent />
  ) : (
    <DefaultErrorComponent error={error} />
  );

export const Route = createFileRoute("/_app/pipelines/$id")({
  validateSearch: pipelineSearchSchema,
  errorComponent: PipelineErrorComponent,
  notFoundComponent: PipelineNotFoundComponent,
  component: PipelinePage,
});
