import type { FC } from "react";

import { ArrowLeftIcon, ImageBrokenIcon } from "@phosphor-icons/react";

import Button from "@galaxy-io/dls/buttons/Button";
import ErrorLayout from "@galaxy-io/dls/layout/ErrorLayout";

import RouterLink from "@/components/RouterLink";

import { createFilamentHref, FilamentPath } from "@/module/paths";

const NotFoundPage: FC = () => (
  <ErrorLayout
    icon={ImageBrokenIcon}
    header="Page not found"
    description="The page you are looking for does not exist"
    actions={
      <Button
        label="Go back to app"
        icon={ArrowLeftIcon}
        href={createFilamentHref(FilamentPath.OBSERVABILITY)}
        as={RouterLink}
      />
    }
  />
);

export default NotFoundPage;
