import { useState } from "react";

import Divider from "@galaxy-io/dls/layout/Divider";
import Flex, { FlexDirection } from "@galaxy-io/dls/layout/Flex";

import { usePipelineCanvasSelection } from "@/pages/pipelines/canvas/hooks/usePipelineCanvasSelection";
import PipelineCanvasPanel from "@/pages/pipelines/canvas/panel/PipelineCanvasPanel";
import { usePipelineCanvasRoutes } from "@/pages/pipelines/canvas/routes/hooks/usePipelineCanvasRoutes";
import { usePipelineCanvasRoutesDraft } from "@/pages/pipelines/canvas/routes/hooks/usePipelineCanvasRoutesDraft";
import PipelineCanvasRoutesList from "@/pages/pipelines/canvas/routes/PipelineCanvasRoutesList";
import PipelineCanvasRoutesToolbar from "@/pages/pipelines/canvas/routes/PipelineCanvasRoutesToolbar";

const PipelineCanvasRoutes = () => {
  const { sinkIds, showPanel } = usePipelineCanvasSelection();
  const [search, setSearch] = useState("");
  const { routes, hasRoutes } = usePipelineCanvasRoutes({ search, sinkIds });
  const draft = usePipelineCanvasRoutesDraft();

  return (
    <>
      <Flex direction={FlexDirection.COLUMN} height="100%">
        <PipelineCanvasRoutesToolbar
          onSearch={setSearch}
          canAddRoute={draft.canOpen}
          onAddRoute={() => draft.open()}
        />
        <Divider />
        <PipelineCanvasRoutesList routes={routes} hasRoutes={hasRoutes} draftState={draft} />
      </Flex>
      {showPanel && <PipelineCanvasPanel />}
    </>
  );
};

export default PipelineCanvasRoutes;
