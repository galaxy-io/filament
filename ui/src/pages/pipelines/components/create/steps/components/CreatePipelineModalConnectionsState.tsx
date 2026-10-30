import type { FC } from "react";

import { PlusIcon } from "@phosphor-icons/react";

import Button from "@galaxy-io/dls/buttons/Button";
import EmptyLayout, { EmptyLayoutSize } from "@galaxy-io/dls/layout/EmptyLayout";
import ErrorLayout, { ErrorLayoutSize } from "@galaxy-io/dls/layout/ErrorLayout";
import Flex, { AlignItems, JustifyContent } from "@galaxy-io/dls/layout/Flex";

import type { ConnectorKind } from "@/gen/ingestion/v1/common_pb";

import { CONNECTOR_KIND_TO_NOUN_MAP } from "@/components/connections/constants";

import { useFilamentFlowOpen } from "@/module/hooks";
import { Flow } from "@/module/types";

import { IS_DEBUG } from "@/constants";

interface CreatePipelineModalConnectionsStateProps {
  message: string;
  error?: Error | null;
  connectorKind: ConnectorKind;
}

const CreatePipelineModalConnectionsState: FC<CreatePipelineModalConnectionsStateProps> = ({
  message,
  error,
  connectorKind,
}) => {
  const openFlow = useFilamentFlowOpen();

  const handleCreateConnection = () => {
    openFlow(Flow.CREATE_CONNECTION, connectorKind);
  };

  const actions = (
    <Button
      label={`Create ${CONNECTOR_KIND_TO_NOUN_MAP[connectorKind]}`}
      icon={PlusIcon}
      onClick={handleCreateConnection}
    />
  );

  return (
    <Flex
      fillWidth
      height="100%"
      minHeight={240}
      alignItems={AlignItems.CENTER}
      justifyContent={JustifyContent.CENTER}
      padding={24}
    >
      {error ? (
        <ErrorLayout
          size={ErrorLayoutSize.SMALL}
          header={message}
          detail={IS_DEBUG ? error.message : undefined}
          actions={actions}
        />
      ) : (
        <EmptyLayout size={EmptyLayoutSize.SMALL} header={message} actions={actions} />
      )}
    </Flex>
  );
};

export default CreatePipelineModalConnectionsState;
