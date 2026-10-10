import type { FC, MouseEvent } from "react";

import type { ConnectorKind } from "@/gen/ingestion/v1/common_pb";
import type { Connection } from "@/gen/ingestion/v1/connections_pb";

import ConnectorTile, {
  ConnectorTileShimmer,
  type ConnectorTileSize,
} from "@/components/connections/ConnectorTile";

import { useFilamentSearchUpdate } from "@/module/hooks";
import type { FilamentLayoutSearch } from "@/module/schemas";

import { createGetConnectionInput, useGetConnectionQuery } from "@/api/queries/connections";

interface PipelineFlowTileProps {
  connectionId: Connection["id"];
  kind: ConnectorKind;
  size: ConnectorTileSize;
}

const PipelineFlowTile: FC<PipelineFlowTileProps> = ({ connectionId, kind, size }) => {
  const updateSearch = useFilamentSearchUpdate<FilamentLayoutSearch>();

  const { data, isLoading } = useGetConnectionQuery({
    input: createGetConnectionInput(connectionId),
  });
  const connection = data?.connection;

  const handleClick = (e: MouseEvent) => {
    e.preventDefault();
    e.stopPropagation();
    updateSearch((prev) => ({ ...prev, connectionId }));
  };

  if (isLoading) {
    return <ConnectorTileShimmer size={size} />;
  }

  return (
    <ConnectorTile
      connector={connection?.connector ?? ""}
      kind={kind}
      size={size}
      onClick={handleClick}
      isDeleted={Boolean(connection?.deletedAt)}
    />
  );
};

export default PipelineFlowTile;
