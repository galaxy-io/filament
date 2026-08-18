import { styled } from "@linaria/react";

import FlexWrapper, {
  AlignItems,
  FlexDirection,
  JustifyContent,
} from "@galaxy-io/dls/containers/FlexWrapper";
import { InputSize } from "@galaxy-io/dls/inputs/Input";
import SelectInput, { type SelectInputOption } from "@galaxy-io/dls/inputs/SelectInput";
import Text from "@galaxy-io/dls/text/Text";

import type { WriteMode } from "@/gen/ingestion/v1/common_pb";
import type { Connection } from "@/gen/ingestion/v1/connections_pb";

import ConnectorTile from "@/pages/connectors/components/ConnectorTile";
import {
  CREATE_PIPELINE_MODAL_SINK_SELECT_WIDTH,
  WRITE_MODE_TO_LABEL_MAP,
} from "@/pages/pipelines/components/create/constants";
import type { CreatePipelineModalSinkRow } from "@/pages/pipelines/components/create/types";

const SinkNameWrapper = styled.div`
  min-width: 0;
  overflow: hidden;
`;

const CreatePipelineModalDeliverySink = ({
  sink,
  onChange,
}: {
  sink: CreatePipelineModalSinkRow;
  onChange: (sinkId: Connection["id"], writeMode: WriteMode) => void;
}) => {
  const options: SelectInputOption[] = sink.writeModeOptions.map((mode) => ({
    id: String(mode),
    label: WRITE_MODE_TO_LABEL_MAP[mode],
    value: mode,
  }));

  return (
    <FlexWrapper
      alignItems={AlignItems.CENTER}
      justifyContent={JustifyContent.SPACE_BETWEEN}
      gap={16}
      padding="12px 16px"
      fillWidth
    >
      <FlexWrapper alignItems={AlignItems.CENTER} gap={10}>
        <ConnectorTile connector={sink.connection.connector} kind={sink.connection.kind} />
        <SinkNameWrapper>
          <Text isEllipsis>{sink.connection.name}</Text>
        </SinkNameWrapper>
      </FlexWrapper>
      <FlexWrapper direction={FlexDirection.COLUMN} grow={0} shrink={0}>
        <SelectInput
          options={options}
          value={options.find((option) => option.value === sink.writeMode) ?? null}
          onChange={(option) => onChange(sink.connection.id, option.value as WriteMode)}
          size={InputSize.LARGE}
          width={CREATE_PIPELINE_MODAL_SINK_SELECT_WIDTH}
        />
      </FlexWrapper>
    </FlexWrapper>
  );
};

export default CreatePipelineModalDeliverySink;
