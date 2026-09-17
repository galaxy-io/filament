import { Fragment } from "react";

import FlexWrapper, { FlexDirection } from "@galaxy-io/dls/containers/FlexWrapper";
import HorizontalDivider from "@galaxy-io/dls/dividers/HorizontalDivider";
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
    <Widget noPadding noHover fillWidth>
      <FlexWrapper direction={FlexDirection.COLUMN} fillWidth>
        {sinks.map((sink, index) => (
          <Fragment key={sink.connection.id}>
            {index > 0 && <HorizontalDivider />}
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
      </FlexWrapper>
    </Widget>
  );
};

export default CreatePipelineModalDeliveryDestinations;
