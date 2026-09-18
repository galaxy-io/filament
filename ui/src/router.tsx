import { ArrowLeftIcon, BugIcon, ImageBrokenIcon } from "@phosphor-icons/react";
import { createRouter, useNavigate } from "@tanstack/react-router";

import Button from "@galaxy-io/dls/buttons/Button";

import ErrorLayout from "@/layouts/ErrorLayout";
import PendingLayout from "@/layouts/PendingLayout";

import { routeTree } from "@/routeTree.gen";

const DEFAULT_PRELOAD = "intent";
const DEFAULT_PRELOAD_STALE_TIME = 0;

export const DefaultErrorComponent = ({ error }: { error: Error }) => {
  return (
    <ErrorLayout
      icon={BugIcon}
      header="Looks like there was a glitch in the matrix"
      message="Please try again later"
      error={error}
    />
  );
};

const DefaultNotFoundComponent = () => {
  const navigate = useNavigate();

  const handleGoToPipelines = () => {
    void navigate({ to: "/pipelines" });
  };

  return (
    <ErrorLayout
      icon={ImageBrokenIcon}
      header="Page not found"
      message="The page you are looking for does not exist"
      actions={<Button label="Go back to app" icon={ArrowLeftIcon} onClick={handleGoToPipelines} />}
    />
  );
};

const DefaultPendingComponent = () => {
  return <PendingLayout />;
};

export const router = createRouter({
  routeTree,
  defaultPreload: DEFAULT_PRELOAD,
  defaultPreloadStaleTime: DEFAULT_PRELOAD_STALE_TIME,
  defaultErrorComponent: DefaultErrorComponent,
  defaultNotFoundComponent: DefaultNotFoundComponent,
  defaultPendingComponent: DefaultPendingComponent,
});

declare module "@tanstack/react-router" {
  interface Register {
    router: typeof router;
  }
}
