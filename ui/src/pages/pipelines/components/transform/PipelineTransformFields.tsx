import Divider from "@galaxy-io/dls/layout/Divider";
import Flex, { AlignItems, FlexDirection } from "@galaxy-io/dls/layout/Flex";

import {
  usePipelineTransformFieldsEnvironment,
  usePipelineTransformFieldsState,
} from "@/pages/pipelines/components/transform/PipelineTransformFieldsProvider";
import PipelineTransformFieldsResourceSteps from "@/pages/pipelines/components/transform/PipelineTransformFieldsResourceSteps";
import PipelineTransformFieldsStepCard from "@/pages/pipelines/components/transform/PipelineTransformFieldsStepCard";

const PipelineTransformFields = () => {
  const { stepsByResource, draft } = usePipelineTransformFieldsState();
  const { resources } = usePipelineTransformFieldsEnvironment();
  const hasSteps = (resource: string) => (stepsByResource.get(resource)?.length ?? 0) > 0;

  return (
    <Flex alignItems={AlignItems.START} direction={FlexDirection.COLUMN} fillWidth>
      {resources.map((resource, index) => (
        <PipelineTransformFieldsResourceSteps
          key={resource}
          resource={resource}
          hasDivider={resources.slice(0, index).some(hasSteps)}
        />
      ))}
      {draft?.id === null && (
        <>
          {resources.some(hasSteps) && <Divider />}
          <PipelineTransformFieldsStepCard draft={draft} />
        </>
      )}
    </Flex>
  );
};

export default PipelineTransformFields;
