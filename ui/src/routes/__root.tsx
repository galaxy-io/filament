import { styled } from "@linaria/react";
import { BugIcon } from "@phosphor-icons/react";
import { createRootRoute, Outlet, useRouter } from "@tanstack/react-router";

import Button from "@galaxy-io/dls/buttons/Button";
import { t } from "@galaxy-io/dls/theme/tokens/t";

import ErrorLayout from "@/layouts/ErrorLayout";

import { createGetAuthConfigQueryOptions } from "@/api/queries/auth";
import { queryClient } from "@/api/queryClient";
import { transport } from "@/api/transport";

const RootComponentWrapper = styled.div`
  display: flex;
  flex-direction: column;

  height: 100vh;
  width: 100vw;
  background-color: ${t.color.background.base};

  ::selection {
    background-color: ${t.color.background.blue};
  }
`;

const RootErrorComponent = ({ error }: { error: Error }) => {
  const router = useRouter();

  return (
    <ErrorLayout
      icon={BugIcon}
      header="Could not reach the server"
      message="Please try again later"
      error={error}
      actions={<Button label="Retry" onClick={() => void router.invalidate()} />}
    />
  );
};

const RootComponent = () => {
  return (
    <RootComponentWrapper>
      <Outlet />
    </RootComponentWrapper>
  );
};

export const Route = createRootRoute({
  beforeLoad: async () => {
    const authConfig = await queryClient.ensureQueryData(
      createGetAuthConfigQueryOptions({ transport }),
    );
    return { authConfig };
  },
  errorComponent: RootErrorComponent,
  component: RootComponent,
});
