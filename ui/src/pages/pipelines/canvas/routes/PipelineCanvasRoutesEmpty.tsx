import { FlowArrowIcon, MagnifyingGlassIcon } from "@phosphor-icons/react";

import Icon, { IconVariant } from "@galaxy-io/dls/icons/Icon";

import EmptyLayout from "@/layouts/EmptyLayout";

interface PipelineCanvasRoutesEmptyProps {
  hasRoutes: boolean;
}

const PipelineCanvasRoutesEmpty = ({ hasRoutes }: PipelineCanvasRoutesEmptyProps) =>
  hasRoutes ? (
    <EmptyLayout
      icon={<Icon component={MagnifyingGlassIcon} variant={IconVariant.TERTIARY} />}
      header="No matching resources"
      message="Try a different search or clear the sink filter."
    />
  ) : (
    <EmptyLayout
      icon={<Icon component={FlowArrowIcon} variant={IconVariant.TERTIARY} />}
      header="No routes"
      message="Connect a source to a sink to route resources."
    />
  );

export default PipelineCanvasRoutesEmpty;
