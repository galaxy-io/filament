import { PencilIcon } from "@phosphor-icons/react";
import { useNavigate } from "@tanstack/react-router";

import Button, {
  ButtonSize,
  ButtonVariant,
} from "@galaxy-io/dls/buttons/Button";
import FlexItem from "@galaxy-io/dls/containers/FlexItem";
import FlexWrapper, {
  AlignItems,
  FlexDirection,
  FlexGap,
} from "@galaxy-io/dls/containers/FlexWrapper";
import Wrapper from "@galaxy-io/dls/containers/Wrapper";
import Tooltip from "@galaxy-io/dls/tooltip/Tooltip";

import type { Connection } from "@/gen/ingestion/v1/connections_pb";

import BaseHeader from "@/layouts/components/BaseHeader";

import ConnectorTile, {
  ConnectorTileSize,
} from "@/pages/connectors/components/ConnectorTile";
import { useConnectorSpec } from "@/pages/connectors/hooks/useConnectorSpec";

import { Flow } from "@/routes/__root";

interface ConnectionDrawerHeaderProps {
  connection: Connection;
  onClose: () => void;
}

const ConnectionDrawerHeader = ({
  connection,
  onClose,
}: ConnectionDrawerHeaderProps) => {
  const navigate = useNavigate();
  const connector = useConnectorSpec(connection.connector, connection.kind);

  const handleEdit = () => {
    void navigate({
      to: ".",
      search: (prev) => ({ ...prev, flow: Flow.EDIT_CONNECTION }),
    });
  };

  return (
    <Wrapper padding="12px 16px">
      <FlexWrapper
        fillWidth
        alignItems={AlignItems.CENTER}
        gap={FlexGap.MEDIUM}
      >
        <FlexItem shrink={0}>
          <ConnectorTile
            connector={connection.connector}
            spec={connector}
            size={ConnectorTileSize.LARGE}
          />
        </FlexItem>
        <FlexWrapper
          fillWidth
          minWidth={0}
          direction={FlexDirection.COLUMN}
          gap={FlexGap.XSMALL}
        >
          <BaseHeader
            title={connection.name}
            description={connector?.displayName || connection.connector}
            onClose={onClose}
            actions={[
              <Button
                icon={PencilIcon}
                variant={ButtonVariant.SECONDARY}
                size={ButtonSize.SMALL}
                onClick={handleEdit}
              />,
            ]}
          />
        </FlexWrapper>
      </FlexWrapper>
    </Wrapper>
  );
};

export default ConnectionDrawerHeader;
