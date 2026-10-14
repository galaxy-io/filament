import type { FC } from "react";

import Divider from "@galaxy-io/dls/layout/Divider";
import Flex, { AlignItems, FlexDirection } from "@galaxy-io/dls/layout/Flex";

import { usePipelineTransformFieldsState } from "@/pages/pipelines/components/transform/PipelineTransformFieldsProvider";
import PipelineTransformFieldsStepCard from "@/pages/pipelines/components/transform/PipelineTransformFieldsStepCard";
import PipelineTransformFieldsSteps from "@/pages/pipelines/components/transform/PipelineTransformFieldsSteps";

const PipelineTransformFields: FC = () => {
  const { steps, draft } = usePipelineTransformFieldsState();

  return (
    <Flex alignItems={AlignItems.START} direction={FlexDirection.COLUMN} fillWidth>
      <PipelineTransformFieldsSteps />
      {draft?.id === null && (
        <>
          {steps.length > 0 && <Divider />}
          <PipelineTransformFieldsStepCard draft={draft} />
        </>
      )}
    </Flex>
  );
};

export default PipelineTransformFields;
