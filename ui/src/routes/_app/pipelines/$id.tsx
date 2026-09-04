import { Code, ConnectError } from "@connectrpc/connect";
import { ArrowLeftIcon, ImageBrokenIcon } from "@phosphor-icons/react";
import { createFileRoute, type ErrorComponentProps, useNavigate } from "@tanstack/react-router";
import z from "zod";

import Button from "@galaxy-io/dls/buttons/Button";
import Icon, { IconVariant } from "@galaxy-io/dls/icons/Icon";

import ErrorLayout from "@/layouts/ErrorLayout";

import PipelinePage from "@/pages/pipelines/PipelinePage";

import { DefaultErrorComponent } from "@/router";

const PipelineNotFoundComponent = () => {
  const navigate = useNavigate();

  const handleGoToPipelines = () => {
    void navigate({ to: "/pipelines" });
  };

  return (
    <ErrorLayout
      icon={<Icon component={ImageBrokenIcon} size={24} variant={IconVariant.SECONDARY} />}
      header="Pipeline not found"
      message="The pipeline you are looking for does not exist"
      actions={
        <Button label="Go back to pipelines" icon={ArrowLeftIcon} onClick={handleGoToPipelines} />
      }
    />
  );
};

const PipelineErrorComponent = ({ error }: ErrorComponentProps) =>
  error instanceof ConnectError && error.code === Code.NotFound ? (
    <PipelineNotFoundComponent />
  ) : (
    <DefaultErrorComponent error={error} />
  );

const searchParams = z.object({
  version: z.coerce.bigint().positive().optional().catch(undefined),
});

export const Route = createFileRoute("/_app/pipelines/$id")({
  validateSearch: searchParams,
  errorComponent: PipelineErrorComponent,
  notFoundComponent: PipelineNotFoundComponent,
  component: PipelinePage,
});
