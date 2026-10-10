import type { FC } from "react";

import { styled } from "@linaria/react";

import CheckboxInput from "@galaxy-io/dls/inputs/CheckboxInput";
import RadioInput from "@galaxy-io/dls/inputs/RadioInput";
import FlexItem from "@galaxy-io/dls/layout/FlexItem";
import Text, { TextVariant } from "@galaxy-io/dls/text/Text";

import { ConnectorKind } from "@/gen/ingestion/v1/common_pb";
import type { Connection } from "@/gen/ingestion/v1/connections_pb";

import ConnectorTile from "@/components/connections/ConnectorTile";

import {
  useCreatePipelineModalActions,
  useCreatePipelineModalState,
} from "@/pages/pipelines/components/create/CreatePipelineModalProvider";
import CreatePipelineModalConnectionRowFrame from "@/pages/pipelines/components/create/steps/components/CreatePipelineModalConnectionRowFrame";

import { NOOP } from "@/constants";

const RowControlWrapper = styled.div`
  display: flex;
  align-items: center;
  flex-shrink: 0;
  pointer-events: none;
`;

interface CreatePipelineModalConnectionRowProps {
  connection: Connection;
  kind: ConnectorKind;
}

const CreatePipelineModalConnectionRow: FC<CreatePipelineModalConnectionRowProps> = ({
  connection,
  kind,
}) => {
  const { sourceConnection, sinkConnections, executionMode } = useCreatePipelineModalState();
  const { selectSource, toggleSink } = useCreatePipelineModalActions();

  const isSource = kind === ConnectorKind.SOURCE;
  const isDisabled = isSource
    ? connection.executionModes.length === 0
    : !connection.executionModes.includes(executionMode);

  const handleClick = () => {
    if (isSource) {
      selectSource(connection);
      return;
    }
    toggleSink(connection);
  };

  return (
    <CreatePipelineModalConnectionRowFrame
      $isDisabled={isDisabled}
      onClick={isDisabled ? undefined : handleClick}
    >
      <RowControlWrapper>
        {isSource ? (
          <RadioInput
            isSelected={sourceConnection?.id === connection.id}
            isDisabled={isDisabled}
            onChange={NOOP}
          />
        ) : (
          <CheckboxInput
            isChecked={sinkConnections.some((sink) => sink.id === connection.id)}
            isDisabled={isDisabled}
            onChange={NOOP}
            ariaLabel={connection.name}
          />
        )}
      </RowControlWrapper>
      <ConnectorTile connector={connection.connector} kind={connection.kind} />
      <FlexItem minWidth={0} overflow="hidden">
        <Text lineClamp={1} variant={isDisabled ? TextVariant.DISABLED : TextVariant.PRIMARY}>
          {connection.name}
        </Text>
      </FlexItem>
    </CreatePipelineModalConnectionRowFrame>
  );
};

export default CreatePipelineModalConnectionRow;
