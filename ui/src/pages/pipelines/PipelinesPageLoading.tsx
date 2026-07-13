import FlexWrapper, { FlexDirection } from "@galaxy-io/dls/containers/FlexWrapper";

import PipelineCardLoading from "@/pages/pipelines/components/PipelineCardLoading";

const LOADING_ROW_COUNT = 8;

const PipelinesPageLoading = () => {
  return (
    <FlexWrapper fillWidth direction={FlexDirection.COLUMN}>
      {Array.from({ length: LOADING_ROW_COUNT }).map((_, index) => (
        <PipelineCardLoading key={index} />
      ))}
    </FlexWrapper>
  );
};

export default PipelinesPageLoading;
