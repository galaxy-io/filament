import type { FC } from "react";

import Flex, { AlignItems } from "@galaxy-io/dls/layout/Flex";
import Text, { TextSize, TextWeight } from "@galaxy-io/dls/text/Text";

import type { Connection } from "@/gen/ingestion/v1/connections_pb";

import ConnectorTile from "@/components/connections/ConnectorTile";
import { ConnectorTileSize } from "@/components/connections/types";

interface ConnectionDrawerHeaderProps {
  connection: Connection;
}

const ConnectionDrawerHeader: FC<ConnectionDrawerHeaderProps> = ({ connection }) => (
  <Flex alignItems={AlignItems.CENTER} gap={12} minWidth={0}>
    <ConnectorTile
      connector={connection.connector}
      kind={connection.kind}
      size={ConnectorTileSize.LARGE}
      isDeleted={!!connection.deletedAt}
    />
    <Text as="h2" size={TextSize.BODY_LG} weight={TextWeight.MEDIUM} lineClamp={1}>
      {connection.name}
    </Text>
  </Flex>
);

export default ConnectionDrawerHeader;
