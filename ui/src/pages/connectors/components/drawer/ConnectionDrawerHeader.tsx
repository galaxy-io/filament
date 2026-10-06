import { create } from "@bufbuild/protobuf";
import { PencilIcon } from "@phosphor-icons/react";
import { useNavigate, useSearch } from "@tanstack/react-router";

import Button, { ButtonSize, ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import Flex, { AlignItems, FlexDirection } from "@galaxy-io/dls/layout/Flex";
import FlexItem from "@galaxy-io/dls/layout/FlexItem";

import { GetConnectionRequestSchema } from "@/gen/ingestion/v1/connections_pb";
import { GetConnectorRequestSchema } from "@/gen/ingestion/v1/connectors_pb";

import { Flow } from "@/layouts/app/types";
import BaseHeader from "@/layouts/components/BaseHeader";

import ConnectorTile, { ConnectorTileSize } from "@/pages/connectors/components/ConnectorTile";

import { useGetConnectionQuery } from "@/api/queries/connections";
import { useGetConnectorQuery } from "@/api/queries/connectors";

const ConnectionDrawerHeader = () => {
  const navigate = useNavigate();
  const { connectionId } = useSearch({ from: "/_app" });

  const { data: connectionData } = useGetConnectionQuery({
    input: create(GetConnectionRequestSchema, { id: connectionId ?? "" }),
    options: { enabled: !!connectionId, retry: false },
  });
  const connection = connectionData?.connection;

  const { data: connectorData } = useGetConnectorQuery({
    input: create(GetConnectorRequestSchema, {
      connector: connection?.connector ?? "",
      kind: connection?.kind,
    }),
    options: { enabled: !!connection },
  });
  const connector = connectorData?.connector;

  const handleEdit = () => {
    void navigate({
      to: ".",
      search: (prev) => ({ ...prev, flow: Flow.EDIT_CONNECTION }),
    });
  };

  if (!connection) return null;

  return (
    <Flex fillWidth alignItems={AlignItems.CENTER} gap={12}>
      <FlexItem shrink={0}>
        <ConnectorTile
          connector={connection.connector}
          kind={connection.kind}
          size={ConnectorTileSize.LARGE}
          isDeleted={!!connection.deletedAt}
        />
      </FlexItem>
      <Flex
        alignItems={AlignItems.START}
        fillWidth
        minWidth={0}
        direction={FlexDirection.COLUMN}
        gap={4}
      >
        <BaseHeader
          title={connection.name}
          description={connector ? connector.displayName || connection.connector : undefined}
          actions={[
            <Button
              key="edit"
              icon={PencilIcon}
              ariaLabel="Edit connection"
              variant={ButtonVariant.SECONDARY}
              size={ButtonSize.SMALL}
              onClick={handleEdit}
              isDisabled={!!connection.deletedAt}
            />,
          ]}
        />
      </Flex>
    </Flex>
  );
};

export default ConnectionDrawerHeader;
