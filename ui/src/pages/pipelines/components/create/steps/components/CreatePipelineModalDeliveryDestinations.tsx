import { type FC, Fragment } from "react";

import Divider from "@galaxy-io/dls/layout/Divider";
import Flex, { AlignItems, FlexDirection } from "@galaxy-io/dls/layout/Flex";
import Widget from "@galaxy-io/dls/widget/Widget";

import {
  useCreatePipelineModalActions,
  useCreatePipelineModalState,
} from "@/pages/pipelines/components/create/CreatePipelineModalProvider";
import CreatePipelineModalDeliverySink from "@/pages/pipelines/components/create/steps/components/CreatePipelineModalDeliverySink";

const CreatePipelineModalDeliveryDestinations: FC = () => {
  const { sinks } = useCreatePipelineModalState();
  const { setSinkWriteMode } = useCreatePipelineModalActions();

  return (
    <Widget isFlush>
      <Flex alignItems={AlignItems.START} direction={FlexDirection.COLUMN} fillWidth>
        {sinks.map((sink, index) => (
          <Fragment key={sink.connection.id}>
            {index > 0 && <Divider />}
            <CreatePipelineModalDeliverySink
              sink={sink}
              onChange={(sinkId, writeMode) => setSinkWriteMode({ sinkId, writeMode })}
            />
          </Fragment>
        ))}
      </Flex>
    </Widget>
  );
};

export default CreatePipelineModalDeliveryDestinations;
