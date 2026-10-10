import type { FC } from "react";

import { ArrowClockwiseIcon, BugIcon } from "@phosphor-icons/react";
import { useQueryErrorResetBoundary } from "@tanstack/react-query";
import { type ErrorComponentProps, useRouter } from "@tanstack/react-router";

import Button from "@galaxy-io/dls/buttons/Button";
import ErrorLayout from "@galaxy-io/dls/layout/ErrorLayout";

import { IS_DEBUG } from "@/constants";

const FilamentErrorComponent: FC<ErrorComponentProps> = ({ error, reset }) => {
  const router = useRouter();
  const { reset: resetQueries } = useQueryErrorResetBoundary();

  const handleRetry = () => {
    resetQueries();
    reset();
    void router.invalidate();
  };

  return (
    <ErrorLayout
      icon={BugIcon}
      header="Looks like there was a glitch in the matrix"
      description="Please try again later"
      detail={IS_DEBUG && error instanceof Error ? error.message : undefined}
      actions={<Button label="Try again" icon={ArrowClockwiseIcon} onClick={handleRetry} />}
    />
  );
};

export default FilamentErrorComponent;
