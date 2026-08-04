import { PencilIcon } from "@phosphor-icons/react";

import Button, { ButtonSize, ButtonVariant } from "@galaxy-io/dls/buttons/Button";
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

import ConnectorTile, { ConnectorTileSize } from "@/pages/connectors/components/ConnectorTile";
import { useConnectorSpec } from "@/pages/connectors/hooks/useConnectorSpec";

interface ConnectionDrawerHeaderProps {
  connection: Connection;
  onClose: () => void;
}

const ConnectionDrawerHeader = ({ connection, onClose }: ConnectionDrawerHeaderProps) => {
  const connector = useConnectorSpec(connection.connector, connection.kind);

  return (
    <Wrapper padding="12px 16px">
      <FlexWrapper fillWidth alignItems={AlignItems.CENTER} gap={FlexGap.MEDIUM}>
        <FlexItem shrink={0}>
          <ConnectorTile
            connector={connection.connector}
            spec={connector}
            size={ConnectorTileSize.LARGE}
          />
        </FlexItem>
        <FlexWrapper fillWidth minWidth={0} direction={FlexDirection.COLUMN} gap={FlexGap.XSMALL}>
          <BaseHeader
            title={connection.name}
            description={connector?.displayName || connection.connector}
            actions={[
              <Tooltip key="edit-connection" body="Edit connection">
                <Button
                  icon={PencilIcon}
                  variant={ButtonVariant.SECONDARY}
                  size={ButtonSize.SMALL}
                  onClick={onClose}
                />
              </Tooltip>,
            ]}
            onClose={onClose}
          />
        </FlexWrapper>
      </FlexWrapper>
    </Wrapper>
  );
};

export default ConnectionDrawerHeader;
