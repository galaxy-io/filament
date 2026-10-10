import type { FC } from "react";

import { BugIcon } from "@phosphor-icons/react";
import type { ErrorComponentProps } from "@tanstack/react-router";

import ErrorLayout from "@galaxy-io/dls/layout/ErrorLayout";

import { IS_DEBUG } from "@/constants";

const FilamentErrorComponent: FC<ErrorComponentProps> = ({ error }) => (
  <ErrorLayout
    icon={BugIcon}
    header="Looks like there was a glitch in the matrix"
    description="Please try again later"
    detail={IS_DEBUG && error instanceof Error ? error.message : undefined}
  />
);

export default FilamentErrorComponent;
