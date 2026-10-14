import type { FC } from "react";

import { ArrowLeftIcon, ImageBrokenIcon } from "@phosphor-icons/react";

import Button from "@galaxy-io/dls/buttons/Button";
import ErrorLayout from "@galaxy-io/dls/layout/ErrorLayout";

import RouterLink from "@/components/RouterLink";

import { createFilamentHref, FilamentPath } from "@/module/paths";

const PipelineNotFoundPage: FC = () => (
  <ErrorLayout
    icon={ImageBrokenIcon}
    header="Pipeline not found"
    description="The pipeline you are looking for does not exist"
    actions={
      <Button
        label="Go back to pipelines"
        icon={ArrowLeftIcon}
        href={createFilamentHref(FilamentPath.PIPELINES)}
        as={RouterLink}
      />
    }
  />
);

export default PipelineNotFoundPage;
