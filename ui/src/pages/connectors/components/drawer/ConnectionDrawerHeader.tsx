import { create } from "@bufbuild/protobuf";
import { PencilIcon } from "@phosphor-icons/react";
import { useNavigate, useSearch } from "@tanstack/react-router";

import Button, { ButtonSize, ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import FlexItem from "@galaxy-io/dls/containers/FlexItem";
import FlexWrapper, {
  AlignItems,
  FlexDirection,
  FlexGap,
} from "@galaxy-io/dls/containers/FlexWrapper";
import Wrapper from "@galaxy-io/dls/containers/Wrapper";

import { GetConnectionRequestSchema } from "@/gen/ingestion/v1/connections_pb";
import { GetConnectorRequestSchema } from "@/gen/ingestion/v1/connectors_pb";

import BaseHeader from "@/layouts/components/BaseHeader";

import ConnectorTile, { ConnectorTileSize } from "@/pages/connectors/components/ConnectorTile";

import { Flow } from "@/routes/__root";

import { useGetConnectionQuery } from "@/api/queries/connections";
import { useGetConnectorQuery } from "@/api/queries/connectors";

interface ConnectionDrawerHeaderProps {
  onClose: () => void;
}

const ConnectionDrawerHeader = ({ onClose }: ConnectionDrawerHeaderProps) => {
  const navigate = useNavigate();
  const { connectionId } = useSearch({ from: "__root__" });

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
    <Wrapper padding="12px 16px">
      <FlexWrapper fillWidth alignItems={AlignItems.CENTER} gap={FlexGap.MEDIUM}>
        <FlexItem shrink={0}>
          <ConnectorTile
            connector={connection.connector}
            kind={connection.kind}
            size={ConnectorTileSize.LARGE}
            isDeleted={!!connection.deletedAt}
          />
        </FlexItem>
        <FlexWrapper fillWidth minWidth={0} direction={FlexDirection.COLUMN} gap={FlexGap.XSMALL}>
          <BaseHeader
            title={connection.name}
            description={connector ? connector.displayName || connection.connector : undefined}
            actions={[
              <Button
                key="edit"
                icon={PencilIcon}
                variant={ButtonVariant.SECONDARY}
                size={ButtonSize.SMALL}
                onClick={handleEdit}
                isDisabled={!!connection.deletedAt}
              />,
            ]}
            onClose={onClose}
          />
        </FlexWrapper>
      </FlexWrapper>
    </Wrapper>
  );
};

export default ConnectionDrawerHeader;
