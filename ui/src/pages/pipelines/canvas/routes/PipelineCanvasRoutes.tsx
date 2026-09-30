import { useState } from "react";

import { styled } from "@linaria/react";

import HorizontalDivider from "@galaxy-io/dls/dividers/HorizontalDivider";
import { useDebouncedValue } from "@galaxy-io/dls/inputs/hooks";

import { usePipelineCanvasSelection } from "@/pages/pipelines/canvas/hooks/usePipelineCanvasSelection";
import PipelineCanvasPanel from "@/pages/pipelines/canvas/panel/PipelineCanvasPanel";
import { usePipelineCanvasRoutes } from "@/pages/pipelines/canvas/routes/hooks/usePipelineCanvasRoutes";
import { usePipelineCanvasRoutesDraft } from "@/pages/pipelines/canvas/routes/hooks/usePipelineCanvasRoutesDraft";
import PipelineCanvasRoutesList from "@/pages/pipelines/canvas/routes/PipelineCanvasRoutesList";
import PipelineCanvasRoutesToolbar from "@/pages/pipelines/canvas/routes/PipelineCanvasRoutesToolbar";

import { LIST_SEARCH_DEBOUNCE_MS } from "@/api/utils";

const RoutesWrapper = styled.div`
  height: 100%;

  display: flex;
  flex-direction: column;
`;

const PipelineCanvasRoutes = () => {
  const { sinkIds, showPanel } = usePipelineCanvasSelection();
  const [search, setSearch] = useState("");
  const debouncedSearch = useDebouncedValue(search, LIST_SEARCH_DEBOUNCE_MS);
  const { routes, hasRoutes } = usePipelineCanvasRoutes({ search: debouncedSearch, sinkIds });
  const draft = usePipelineCanvasRoutesDraft();

  return (
    <>
      <RoutesWrapper>
        <PipelineCanvasRoutesToolbar
          search={search}
          onSearchChange={setSearch}
          canAddRoute={draft.canOpen}
          onAddRoute={() => draft.open()}
        />
        <HorizontalDivider />
        <PipelineCanvasRoutesList routes={routes} hasRoutes={hasRoutes} draftState={draft} />
      </RoutesWrapper>
      {showPanel && <PipelineCanvasPanel />}
    </>
  );
};

export default PipelineCanvasRoutes;
