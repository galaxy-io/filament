import type { FC } from "react";

import { PlusIcon } from "@phosphor-icons/react";

import Button from "@galaxy-io/dls/buttons/Button";
import EmptyLayout, { EmptyLayoutSize } from "@galaxy-io/dls/layout/EmptyLayout";
import Flex, { AlignItems, JustifyContent } from "@galaxy-io/dls/layout/Flex";

import type { ConnectorKind } from "@/gen/ingestion/v1/common_pb";

import { CONNECTOR_KIND_TO_NOUN_MAP } from "@/components/connections/constants";

import { useFilamentFlowOpen } from "@/module/hooks";
import { Flow } from "@/module/types";

interface PipelineCanvasConnectionSelectorEmptyProps {
  message: string;
  connectorKind: ConnectorKind;
}

const PipelineCanvasConnectionSelectorEmpty: FC<PipelineCanvasConnectionSelectorEmptyProps> = ({
  message,
  connectorKind,
}) => {
  const openFlow = useFilamentFlowOpen();

  const handleCreateConnection = () => {
    openFlow(Flow.CREATE_CONNECTION, connectorKind);
  };

  return (
    <Flex
      fillWidth
      height="100%"
      minHeight={240}
      alignItems={AlignItems.CENTER}
      justifyContent={JustifyContent.CENTER}
      padding={24}
    >
      <EmptyLayout
        size={EmptyLayoutSize.SMALL}
        header={message}
        actions={
          <Button
            label={`Create ${CONNECTOR_KIND_TO_NOUN_MAP[connectorKind]}`}
            icon={PlusIcon}
            onClick={handleCreateConnection}
          />
        }
      />
    </Flex>
  );
};

export default PipelineCanvasConnectionSelectorEmpty;
