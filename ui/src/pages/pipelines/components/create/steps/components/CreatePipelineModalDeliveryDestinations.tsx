import { Fragment } from "react";

import Divider from "@galaxy-io/dls/layout/Divider";
import Flex, { AlignItems, FlexDirection } from "@galaxy-io/dls/layout/Flex";
import Widget from "@galaxy-io/dls/widget/Widget";

import { CreatePipelineModalActionType } from "@/pages/pipelines/components/create/actions";
import {
  useCreatePipelineModalDispatch,
  useCreatePipelineModalState,
} from "@/pages/pipelines/components/create/CreatePipelineModalProvider";
import CreatePipelineModalDeliverySink from "@/pages/pipelines/components/create/steps/components/CreatePipelineModalDeliverySink";

const CreatePipelineModalDeliveryDestinations = () => {
  const { sinks } = useCreatePipelineModalState();
  const dispatch = useCreatePipelineModalDispatch();

  return (
    <Widget
      isFlush /* @dls-migrate widget.fillWidth: Grow the card with a `FlexItem` or a `Grid` track. */
      fillWidth
    >
      <Flex alignItems={AlignItems.START} direction={FlexDirection.COLUMN} fillWidth>
        {sinks.map((sink, index) => (
          <Fragment key={sink.connection.id}>
            {index > 0 && <Divider />}
            <CreatePipelineModalDeliverySink
              sink={sink}
              onChange={(sinkId, writeMode) =>
                dispatch({
                  type: CreatePipelineModalActionType.SET_SINK_WRITE_MODE,
                  payload: { sinkId, writeMode },
                })
              }
            />
          </Fragment>
        ))}
      </Flex>
    </Widget>
  );
};

export default CreatePipelineModalDeliveryDestinations;
