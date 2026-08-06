import { useEffect, useLayoutEffect, useRef, useState } from "react";

import { styled } from "@linaria/react";
import { useNodeId, useReactFlow, useUpdateNodeInternals } from "@xyflow/react";

import Badge, { BadgeVariant } from "@galaxy-io/dls/badge/Badge";
import { InputSize } from "@galaxy-io/dls/inputs/Input";
import TextInput from "@galaxy-io/dls/inputs/TextInput";

import {
  PIPELINE_CANVAS_NODE_HANDLE_SLOT_SIZE,
  PIPELINE_CANVAS_NODE_PADDING,
} from "@/pages/pipelines/canvas/nodes/constants";
import PipelineCanvasNodeCollapsibleIsland from "@/pages/pipelines/canvas/nodes/PipelineCanvasNodeCollapsibleIsland";
import PipelineCanvasNodeSourceIslandResourceList from "@/pages/pipelines/canvas/nodes/PipelineCanvasNodeSourceIslandResourceList";
import {
  removePipelineCanvasNodeMeasurements,
  setPipelineCanvasNodeMeasurements,
} from "@/pages/pipelines/canvas/nodes/utils";
import type { PipelineCanvasNodeResourceInfo } from "@/pages/pipelines/canvas/types";

import { isSearchMatch } from "@/utils/search";

const BadgeSlot = styled.span`
  width: ${PIPELINE_CANVAS_NODE_HANDLE_SLOT_SIZE}px;
  flex-shrink: 0;

  display: flex;
  align-items: center;
  justify-content: center;
`;

const SearchSection = styled.div`
  padding: ${PIPELINE_CANVAS_NODE_PADDING}px;
`;

const ResourceList = styled.div`
  display: flex;
  flex-direction: column;
  gap: 2px;
  padding: ${PIPELINE_CANVAS_NODE_PADDING}px ${PIPELINE_CANVAS_NODE_PADDING}px
    ${PIPELINE_CANVAS_NODE_PADDING}px 12px;
`;

const roundMeasurement = (value: number) => Math.round(value * 100) / 100;

interface PipelineCanvasNodeSourceIslandProps {
  resources: PipelineCanvasNodeResourceInfo[];
  error: Error | null;
  isLoading: boolean;
  isSelected?: boolean;
  isOpen: boolean;
  onToggle: () => void;
}

interface PipelineCanvasNodeSourceIslandState {
  search: string;
}

const DEFAULT_STATE: PipelineCanvasNodeSourceIslandState = {
  search: "",
};

const PipelineCanvasNodeSourceIsland = ({
  resources,
  error,
  isLoading,
  isSelected,
  isOpen,
  onToggle,
}: PipelineCanvasNodeSourceIslandProps) => {
  const nodeId = useNodeId();
  const updateNodeInternals = useUpdateNodeInternals();
  const { getZoom } = useReactFlow();
  const bodyRef = useRef<HTMLDivElement>(null);
  const badgeRef = useRef<HTMLSpanElement>(null);
  const [state, setState] = useState<PipelineCanvasNodeSourceIslandState>(DEFAULT_STATE);

  const matchedResources = resources.filter((resource) =>
    isSearchMatch(state.search, resource.name),
  );
  const isInitialLoad = isLoading && resources.length === 0;
  const hasRows = !isInitialLoad && !error && matchedResources.length > 0;
  const renderedNames = new Set(hasRows ? matchedResources.map((resource) => resource.name) : []);
  const hiddenConnectedResources = resources.filter(
    (resource) => resource.isConnected && !renderedNames.has(resource.name),
  );
  const connectedCount = resources.filter((resource) => resource.isConnected).length;

  const publishMeasurements = () => {
    const bodyElement = bodyRef.current;
    const nodeElement = bodyElement?.closest(".react-flow__node");
    const zoom = getZoom();
    if (!nodeId || !bodyElement || !nodeElement || zoom <= 0) return;

    const nodeRect = nodeElement.getBoundingClientRect();
    const badgeElement = badgeRef.current?.firstElementChild ?? badgeRef.current;
    const badgeRect = badgeElement?.getBoundingClientRect() ?? null;
    const toNodeX = (clientX: number) => roundMeasurement((clientX - nodeRect.left) / zoom);
    const toNodeY = (clientY: number) => roundMeasurement((clientY - nodeRect.top) / zoom);
    const bodyRect = isOpen ? bodyElement.getBoundingClientRect() : null;

    setPipelineCanvasNodeMeasurements(nodeId, {
      bodyTop: bodyRect ? toNodeY(bodyRect.top) : 0,
      bodyBottom: bodyRect ? toNodeY(bodyRect.bottom) : 0,
      badgeAnchorX: badgeRect ? toNodeX(badgeRect.right) : null,
      badgeAnchorY: badgeRect ? toNodeY(badgeRect.top + badgeRect.height / 2) : null,
      hiddenHandleIds: hiddenConnectedResources.map((resource) => resource.name),
    });
  };

  const syncNodeInternals = () => {
    if (nodeId) {
      updateNodeInternals(nodeId);
    }
    publishMeasurements();
  };

  const handleSearchChange = (search: string) => {
    setState((prev) => ({ ...prev, search }));
    if (nodeId) {
      updateNodeInternals(nodeId);
    }
  };

  useLayoutEffect(() => {
    publishMeasurements();
  });

  useEffect(() => {
    return () => {
      if (nodeId) {
        removePipelineCanvasNodeMeasurements(nodeId);
      }
    };
  }, [nodeId]);

  return (
    <PipelineCanvasNodeCollapsibleIsland
      isOpen={isOpen}
      onToggle={onToggle}
      isSelected={isSelected}
      onBodyScroll={syncNodeInternals}
      bodyRef={bodyRef}
      title="Resources"
      bodyHeader={
        <SearchSection className="nodrag">
          <TextInput
            placeholder="Search"
            value={state.search}
            onChange={handleSearchChange}
            size={InputSize.LARGE}
            fillWidth
          />
        </SearchSection>
      }
      trailing={
        connectedCount > 0 ? (
          <BadgeSlot ref={badgeRef}>
            <Badge count={connectedCount} variant={BadgeVariant.PRIMARY_ALT} />
          </BadgeSlot>
        ) : undefined
      }
    >
      <ResourceList>
        <PipelineCanvasNodeSourceIslandResourceList
          resources={matchedResources}
          hiddenResources={hiddenConnectedResources}
          error={error}
          isInitialLoad={isInitialLoad}
          emptyMessage={state.search ? "No resources match your search" : "No resources found"}
        />
      </ResourceList>
    </PipelineCanvasNodeCollapsibleIsland>
  );
};

export default PipelineCanvasNodeSourceIsland;
