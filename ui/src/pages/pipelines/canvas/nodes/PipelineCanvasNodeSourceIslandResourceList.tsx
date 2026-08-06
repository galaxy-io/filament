import { styled } from "@linaria/react";
import { WarningCircleIcon } from "@phosphor-icons/react";
import { Position } from "@xyflow/react";

import FlexWrapper from "@galaxy-io/dls/containers/FlexWrapper";
import Icon, { IconVariant } from "@galaxy-io/dls/icons/Icon";
import Text, { TextSize, TextVariant } from "@galaxy-io/dls/text/Text";
import TextShimmer from "@galaxy-io/dls/text/TextShimmer";

import { ConnectorKind } from "@/gen/ingestion/v1/common_pb";

import EmptyLayout from "@/layouts/EmptyLayout";
import ErrorLayout from "@/layouts/ErrorLayout";

import PipelineCanvasNodeHandle from "@/pages/pipelines/canvas/nodes/PipelineCanvasNodeHandle";
import type { PipelineCanvasNodeResourceInfo } from "@/pages/pipelines/canvas/types";

const ResourceRow = styled.div`
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 4px;
`;

const HiddenRows = styled.div`
  display: none;
`;

const SHIMMER_COUNT = 5;

const ResourceListShimmer = () => (
  <>
    {Array.from({ length: SHIMMER_COUNT }).map((_, index) => (
      // biome-ignore lint/suspicious/noArrayIndexKey: static placeholder rows with no identity
      <TextShimmer key={index} height={16} width="100%" />
    ))}
  </>
);

interface PipelineCanvasNodeSourceIslandResourceListProps {
  resources: PipelineCanvasNodeResourceInfo[];
  hiddenResources: PipelineCanvasNodeResourceInfo[];
  error: Error | null;
  isInitialLoad: boolean;
  emptyMessage: string;
}

const ResourceListRow = ({ resource }: { resource: PipelineCanvasNodeResourceInfo }) => (
  <ResourceRow>
    <Text
      size={TextSize.BODY_SM}
      variant={resource.isConnected ? TextVariant.PRIMARY : TextVariant.TERTIARY}
      isMonospace
    >
      {resource.name}
    </Text>
    <PipelineCanvasNodeHandle
      id={resource.name}
      kind={ConnectorKind.SOURCE}
      position={Position.Right}
      isConnected={resource.isConnected}
    />
  </ResourceRow>
);

const PipelineCanvasNodeSourceIslandResourceList = ({
  resources,
  hiddenResources,
  error,
  isInitialLoad,
  emptyMessage,
}: PipelineCanvasNodeSourceIslandResourceListProps) => {
  const renderContent = () => {
    if (isInitialLoad) {
      return <ResourceListShimmer />;
    }

    if (error) {
      return (
        <FlexWrapper padding={"20px 16px"} fillWidth>
          <ErrorLayout
            icon={<Icon component={WarningCircleIcon} size={20} variant={IconVariant.ERROR} />}
            message="Failed to load resources"
            error={error}
          />
        </FlexWrapper>
      );
    }

    if (!resources.length) {
      return (
        <FlexWrapper padding={"20px 16px"} fillWidth>
          <EmptyLayout message={emptyMessage} />
        </FlexWrapper>
      );
    }

    return resources.map((resource) => <ResourceListRow key={resource.name} resource={resource} />);
  };

  return (
    <>
      {renderContent()}
      {hiddenResources.length > 0 && (
        <HiddenRows>
          {hiddenResources.map((resource) => (
            <ResourceListRow key={resource.name} resource={resource} />
          ))}
        </HiddenRows>
      )}
    </>
  );
};

export default PipelineCanvasNodeSourceIslandResourceList;
