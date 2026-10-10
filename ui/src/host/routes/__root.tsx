import type { FC } from "react";

import { styled } from "@linaria/react";
import { BugIcon } from "@phosphor-icons/react";
import {
  createRootRoute,
  type ErrorComponentProps,
  Outlet,
  useRouter,
} from "@tanstack/react-router";

import Button from "@galaxy-io/dls/buttons/Button";
import ErrorLayout from "@galaxy-io/dls/layout/ErrorLayout";
import { t } from "@galaxy-io/dls/theme/tokens/t";

import { queryClient } from "@/host/api/queryClient";
import { transport } from "@/host/api/transport";

import { createGetAuthConfigQueryOptions } from "@/api/queries/auth";

import { IS_DEBUG } from "@/constants";

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

const RootErrorComponent: FC<ErrorComponentProps> = ({ error }) => {
  const router = useRouter();

  return (
    <ErrorLayout
      icon={BugIcon}
      header="Could not reach the server"
      description="Please try again later"
      detail={IS_DEBUG ? error.message : undefined}
      actions={<Button label="Retry" onClick={() => void router.invalidate()} />}
    />
  );
};

const RootComponent: FC = () => {
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
