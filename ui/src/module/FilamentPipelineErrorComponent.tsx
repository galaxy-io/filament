import type { FC } from "react";

import { Code, ConnectError } from "@connectrpc/connect";
import type { ErrorComponentProps } from "@tanstack/react-router";

import PipelineNotFoundPage from "@/pages/pipelines/PipelineNotFoundPage";

import FilamentErrorComponent from "@/module/FilamentErrorComponent";

const FilamentPipelineErrorComponent: FC<ErrorComponentProps> = (props) =>
  props.error instanceof ConnectError && props.error.code === Code.NotFound ? (
    <PipelineNotFoundPage />
  ) : (
    <FilamentErrorComponent {...props} />
  );

export default FilamentPipelineErrorComponent;
