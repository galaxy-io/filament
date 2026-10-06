import pluralize from "pluralize";

import Chip, { ChipSize, ChipVariant } from "@galaxy-io/dls/chips/Chip";
import Alert from "@galaxy-io/dls/feedback/Alert";
import Box from "@galaxy-io/dls/layout/Box";
import Flex, { AlignItems, JustifyContent } from "@galaxy-io/dls/layout/Flex";
import Link, { LinkUnderline } from "@galaxy-io/dls/links/Link";
import Text, { TextVariant, TextWeight } from "@galaxy-io/dls/text/Text";
import { t } from "@galaxy-io/dls/theme/tokens/t";

import type { ConnectorKind } from "@/gen/ingestion/v1/common_pb";
import type { ConnectorMaturity, ConnectorSpec } from "@/gen/ingestion/v1/connectors_pb";

import ConnectionKindChip from "@/pages/connectors/components/ConnectionKindChip";
import { getConnectorFamilyName } from "@/pages/connectors/components/create/utils";
import {
  CONNECTOR_KIND_TO_LABEL_MAP,
  CONNECTOR_MATURITY_TO_STATUS_MAP,
} from "@/pages/connectors/constants";

import { DOCUMENTATION_URL } from "@/constants";

interface ConnectionFormMaturityAlertProps {
  connectorName: ConnectorSpec["name"];
  connectorKind: ConnectorKind;
  connectorMaturity: ConnectorMaturity;
  connectorApiVersion: ConnectorSpec["apiVersion"];
}

const createDocsPath = (connectorName: ConnectorSpec["name"], connectorKind: ConnectorKind) => {
  const kindSegment = pluralize(CONNECTOR_KIND_TO_LABEL_MAP[connectorKind]).toLowerCase();
  return `/pages/connectors/${kindSegment}/${getConnectorFamilyName(connectorName).toLowerCase()}`;
};

const ConnectionFormMaturityAlert = ({
  connectorName,
  connectorKind,
  connectorMaturity,
  connectorApiVersion,
}: ConnectionFormMaturityAlertProps) => {
  const status = CONNECTOR_MATURITY_TO_STATUS_MAP[connectorMaturity];
  if (!status) return null;

  return (
    <Box fillWidth>
      <Alert>
        <Flex
          alignItems={AlignItems.CENTER}
          justifyContent={JustifyContent.SPACE_BETWEEN}
          gap={8}
          height={t.font.sans.height.body_md}
        >
          <Flex alignItems={AlignItems.CENTER} gap={4}>
            This
            <ConnectionKindChip kind={connectorKind} size={ChipSize.SMALL} />
            {connectorApiVersion && (
              <>
                at
                <Chip
                  label={connectorApiVersion}
                  variant={ChipVariant.TERTIARY}
                  size={ChipSize.SMALL}
                />
              </>
            )}
            is
            <Text variant={TextVariant.PRIMARY} weight={TextWeight.MEDIUM}>
              {status}.
            </Text>
          </Flex>
          <Link
            href={`${DOCUMENTATION_URL}${createDocsPath(connectorName, connectorKind)}`}
            underline={LinkUnderline.HOVER}
            isExternal
          >
            Setup guide
          </Link>
        </Flex>
      </Alert>
    </Box>
  );
};

export default ConnectionFormMaturityAlert;
