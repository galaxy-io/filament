import type { FC } from "react";

import Divider from "@galaxy-io/dls/layout/Divider";
import Flex, { AlignItems, FlexDirection } from "@galaxy-io/dls/layout/Flex";
import { Orientation } from "@galaxy-io/dls/theme/enums";

import { ConnectorKind } from "@/gen/ingestion/v1/common_pb";

import { useCreatePipelineModalState } from "@/pages/pipelines/components/create/CreatePipelineModalProvider";
import CreatePipelineModalConnectionsExecutionMode from "@/pages/pipelines/components/create/steps/components/CreatePipelineModalConnectionsExecutionMode";
import CreatePipelineModalConnectionsPane from "@/pages/pipelines/components/create/steps/components/CreatePipelineModalConnectionsPane";

const CreatePipelineModalConnections: FC = () => {
  const { supportedExecutionModes } = useCreatePipelineModalState();

  return (
    <Flex
      direction={FlexDirection.COLUMN}
      alignItems={AlignItems.STRETCH}
      grow={1}
      basis={0}
      minHeight={0}
    >
      <Flex alignItems={AlignItems.STRETCH} grow={1} basis={0} minHeight={0}>
        <CreatePipelineModalConnectionsPane kind={ConnectorKind.SOURCE} />
        <Divider orientation={Orientation.VERTICAL} />
        <CreatePipelineModalConnectionsPane kind={ConnectorKind.SINK} />
      </Flex>
      {supportedExecutionModes.length > 1 && (
        <>
          <Divider />
          <Flex alignItems={AlignItems.START} padding={12} shrink={0} fillWidth>
            <CreatePipelineModalConnectionsExecutionMode />
          </Flex>
        </>
      )}
    </Flex>
  );
};

export default CreatePipelineModalConnections;
