import { FlowArrowIcon, XIcon } from "@phosphor-icons/react";
import { useNavigate } from "@tanstack/react-router";

import FlexWrapper, { AlignItems, FlexGap } from "@galaxy-io/dls/containers/FlexWrapper";
import Icon, { IconVariant, IconWeight } from "@galaxy-io/dls/icons/Icon";

import ConnectorTile, {
  ConnectorOverflowTile,
  ConnectorTileEmpty,
} from "@/pages/connectors/components/ConnectorTile";

const PIPELINE_FLOW_MAX_VISIBLE_SINKS = 3;

export interface PipelineFlowConnection {
  connectionId: string;
  connector: string;
}

interface PipelineFlowProps {
  source?: PipelineFlowConnection;
  sinks?: PipelineFlowConnection[];
  hasEdges?: boolean;
}

const PipelineFlow = ({ source, sinks = [], hasEdges = true }: PipelineFlowProps) => {
  const navigate = useNavigate();

  const visibleSinks = sinks.slice(0, PIPELINE_FLOW_MAX_VISIBLE_SINKS);
  const overflowCount = sinks.length - visibleSinks.length;

  const hasSource = !!source && source.connectionId.length > 0;
  const hasSinks = sinks.length > 0;
  const isLinked = hasSource && hasSinks && hasEdges;

  const handleConnectionClick = (connectionId: string, e: React.MouseEvent) => {
    e.preventDefault();
    e.stopPropagation();
    navigate({
      to: ".",
      search: (prev) => ({ ...prev, connectionId }),
    });
  };

  return (
    <FlexWrapper alignItems={AlignItems.CENTER} gap={FlexGap.SMALL}>
      {hasSource ? (
        <ConnectorTile
          connector={source.connector}
          onClick={(e) => handleConnectionClick(source.connectionId, e)}
        />
      ) : (
        <ConnectorTileEmpty />
      )}
      <Icon
        component={isLinked ? FlowArrowIcon : XIcon}
        variant={isLinked ? IconVariant.PRIMARY : IconVariant.ERROR}
        size={16}
        weight={IconWeight.REGULAR}
      />
      {hasSinks ? (
        <FlexWrapper alignItems={AlignItems.CENTER} gap={FlexGap.XSMALL}>
          {visibleSinks.map((sink, index) => (
            <ConnectorTile
              // biome-ignore lint/suspicious/noArrayIndexKey: two sink nodes can share a connection
              key={`${sink.connectionId}-${index}`}
              connector={sink.connector}
              onClick={(e) => handleConnectionClick(sink.connectionId, e)}
            />
          ))}
          {overflowCount > 0 && <ConnectorOverflowTile count={overflowCount} />}
        </FlexWrapper>
      ) : (
        <ConnectorTileEmpty />
      )}
    </FlexWrapper>
  );
};

export default PipelineFlow;
