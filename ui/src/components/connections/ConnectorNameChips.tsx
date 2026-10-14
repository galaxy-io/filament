import type { FC } from "react";

import { create } from "@bufbuild/protobuf";

import Chip, { ChipSize, ChipVariant } from "@galaxy-io/dls/chips/Chip";
import Flex, { AlignItems } from "@galaxy-io/dls/layout/Flex";

import type { Connection } from "@/gen/ingestion/v1/connections_pb";
import { GetConnectorRequestSchema } from "@/gen/ingestion/v1/connectors_pb";

import {
  formatConnectorName,
  getConnectorFamilyName,
  getConnectorVariantName,
} from "@/components/connections/utils";

import { useGetConnectorQuery } from "@/api/queries/connectors";

interface ConnectorNameChipsProps {
  connector: Connection["connector"];
  kind: Connection["kind"];
}

const ConnectorNameChips: FC<ConnectorNameChipsProps> = ({ connector, kind }) => {
  const { data } = useGetConnectorQuery({
    input: create(GetConnectorRequestSchema, { connector, kind }),
    options: { enabled: connector !== "" },
  });

  const variant = getConnectorVariantName(connector);

  return (
    <Flex alignItems={AlignItems.CENTER} gap={8} minWidth={0}>
      <Chip
        label={
          data?.connector ? formatConnectorName(data.connector) : getConnectorFamilyName(connector)
        }
        variant={ChipVariant.SECONDARY}
        size={ChipSize.SMALL}
      />
      {variant && <Chip label={variant} variant={ChipVariant.SECONDARY} size={ChipSize.SMALL} />}
    </Flex>
  );
};

export default ConnectorNameChips;
