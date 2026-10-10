import type { FC } from "react";

import Chip, { ChipSize, ChipVariant } from "@galaxy-io/dls/chips/Chip";
import Alert from "@galaxy-io/dls/feedback/Alert";
import Box from "@galaxy-io/dls/layout/Box";
import Flex, { AlignItems, JustifyContent } from "@galaxy-io/dls/layout/Flex";
import Link, { LinkUnderline } from "@galaxy-io/dls/links/Link";
import Span, { SpanVariant } from "@galaxy-io/dls/text/Span";
import { TextWeight } from "@galaxy-io/dls/text/Text";
import { t } from "@galaxy-io/dls/theme/tokens/t";

import type { ConnectorKind } from "@/gen/ingestion/v1/common_pb";
import type { ConnectorMaturity, ConnectorSpec } from "@/gen/ingestion/v1/connectors_pb";

import ConnectionKindChip from "@/components/connections/ConnectionKindChip";
import { CONNECTOR_KIND_TO_PLURAL_NOUN_MAP } from "@/components/connections/constants";
import { getConnectorFamilyName } from "@/components/connections/utils";

import { CONNECTOR_MATURITY_TO_STATUS_MAP } from "@/pages/connections/constants";

import { DOCUMENTATION_URL } from "@/constants";

interface ConnectionFormMaturityAlertProps {
  connectorName: ConnectorSpec["name"];
  connectorKind: ConnectorKind;
  connectorMaturity: ConnectorMaturity;
  connectorApiVersion: ConnectorSpec["apiVersion"];
}

const createDocsPath = (connectorName: ConnectorSpec["name"], connectorKind: ConnectorKind) => {
  const kindSegment = CONNECTOR_KIND_TO_PLURAL_NOUN_MAP[connectorKind];
  return `/pages/connectors/${kindSegment}/${getConnectorFamilyName(connectorName).toLowerCase()}`;
};

const ConnectionFormMaturityAlert: FC<ConnectionFormMaturityAlertProps> = ({
  connectorName,
  connectorKind,
  connectorMaturity,
  connectorApiVersion,
}) => {
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
            <Span>
              <Span variant={SpanVariant.PRIMARY} weight={TextWeight.MEDIUM}>
                {status}
              </Span>
              .
            </Span>
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
