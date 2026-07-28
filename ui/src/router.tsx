import { ArrowLeftIcon, BugIcon, ImageBrokenIcon } from "@phosphor-icons/react";
import { createRouter, useNavigate } from "@tanstack/react-router";

import Button from "@galaxy-io/dls/buttons/Button";
import Icon, { IconVariant } from "@galaxy-io/dls/icons/Icon";

import ErrorLayout from "@/layouts/ErrorLayout";
import PendingLayout from "@/layouts/PendingLayout";

import { routeTree } from "@/routeTree.gen";

const DefaultErrorComponent = ({ error }: { error: Error }) => {
  return (
    <ErrorLayout
      icon={<Icon component={BugIcon} size={24} variant={IconVariant.ERROR} />}
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
      icon={<Icon component={ImageBrokenIcon} size={24} variant={IconVariant.SECONDARY} />}
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
  defaultErrorComponent: DefaultErrorComponent,
  defaultNotFoundComponent: DefaultNotFoundComponent,
  defaultPendingComponent: DefaultPendingComponent,
});

declare module "@tanstack/react-router" {
  interface Register {
    router: typeof router;
  }
}
