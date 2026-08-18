import Accordion, { AccordionSize } from "@galaxy-io/dls/accordion/Accordion";
import FlexWrapper, { FlexDirection, FlexGap } from "@galaxy-io/dls/containers/FlexWrapper";
import Text, { TextSize, TextVariant, TextWeight } from "@galaxy-io/dls/text/Text";

import { CreatePipelineModalActionType } from "@/pages/pipelines/components/create/actions";
import {
  useCreatePipelineModalDispatch,
  useCreatePipelineModalState,
} from "@/pages/pipelines/components/create/CreatePipelineModalProvider";
import PipelineScheduleFields from "@/pages/pipelines/components/schedule/PipelineScheduleFields";
import PipelineWorkerResourcesFields from "@/pages/pipelines/components/worker/PipelineWorkerResourcesFields";
import { formatPipelineScheduleSummary } from "@/pages/pipelines/settings/utils";

interface CreatePipelineModalDeliverySectionProps {
  header: string;
  children: React.ReactNode;
}

const CreatePipelineModalDeliverySection = ({
  header,
  children,
}: CreatePipelineModalDeliverySectionProps) => (
  <FlexWrapper direction={FlexDirection.COLUMN} gap={FlexGap.MEDIUM} fillWidth>
    <FlexWrapper direction={FlexDirection.COLUMN} gap={2} fillWidth>
      <Text size={TextSize.BODY_LG} weight={TextWeight.MEDIUM}>
        {header}
      </Text>
    </FlexWrapper>
    {children}
  </FlexWrapper>
);

const CreatePipelineModalDeliverySchedule = () => {
  const { schedule } = useCreatePipelineModalState();
  const dispatch = useCreatePipelineModalDispatch();

  const summary = formatPipelineScheduleSummary(schedule);

  return (
    <FlexWrapper direction={FlexDirection.COLUMN} gap={FlexGap.MEDIUM} fillWidth>
      <PipelineScheduleFields
        state={schedule}
        onChange={(partial) =>
          dispatch({
            type: CreatePipelineModalActionType.SET_SCHEDULE,
            payload: partial,
          })
        }
      />
      {schedule.isEnabled && summary && (
        <Text size={TextSize.BODY_SM} variant={TextVariant.SUCCESS}>
          {summary}
        </Text>
      )}
    </FlexWrapper>
  );
};

const CreatePipelineModalDeliveryAdvanced = () => {
  const { workerResources } = useCreatePipelineModalState();
  const dispatch = useCreatePipelineModalDispatch();

  return (
    <Accordion header="Worker resources" padding="16px" size={AccordionSize.LARGE}>
      <PipelineWorkerResourcesFields
        state={workerResources}
        onChange={(payload) =>
          dispatch({
            type: CreatePipelineModalActionType.SET_WORKER_RESOURCES,
            payload,
          })
        }
      />
    </Accordion>
  );
};

const CreatePipelineModalDelivery = () => {
  return (
    <FlexWrapper direction={FlexDirection.COLUMN} gap={FlexGap.LARGE} fillWidth>
      <CreatePipelineModalDeliverySection header="Schedule">
        <CreatePipelineModalDeliverySchedule />
      </CreatePipelineModalDeliverySection>
      <CreatePipelineModalDeliveryAdvanced />
    </FlexWrapper>
  );
};

export default CreatePipelineModalDelivery;
