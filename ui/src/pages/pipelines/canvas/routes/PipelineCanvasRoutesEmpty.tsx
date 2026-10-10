import type { FC } from "react";

import { FlowArrowIcon, MagnifyingGlassIcon } from "@phosphor-icons/react";

import EmptyLayout from "@galaxy-io/dls/layout/EmptyLayout";

interface PipelineCanvasRoutesEmptyProps {
  hasRoutes: boolean;
}

const PipelineCanvasRoutesEmpty: FC<PipelineCanvasRoutesEmptyProps> = ({ hasRoutes }) =>
  hasRoutes ? (
    <EmptyLayout
      icon={MagnifyingGlassIcon}
      header="No matching resources"
      description="Try a different search or clear the sink filter."
    />
  ) : (
    <EmptyLayout
      icon={FlowArrowIcon}
      header="No routes"
      description="Connect a source to a sink to route resources."
    />
  );

export default PipelineCanvasRoutesEmpty;
