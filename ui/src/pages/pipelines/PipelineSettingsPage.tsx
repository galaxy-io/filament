import { create } from "@bufbuild/protobuf";
import { styled } from "@linaria/react";
import { useParams } from "@tanstack/react-router";

import FlexWrapper, {
  AlignItems,
  FlexDirection,
  JustifyContent,
} from "@galaxy-io/dls/containers/FlexWrapper";
import Wrapper from "@galaxy-io/dls/containers/Wrapper";
import HorizontalDivider from "@galaxy-io/dls/dividers/HorizontalDivider";
import CopyInput from "@galaxy-io/dls/inputs/CopyInput";
import { InputSize } from "@galaxy-io/dls/inputs/Input";
import Text, { TextWeight } from "@galaxy-io/dls/text/Text";
import { withTheme } from "@galaxy-io/dls/theme/GalaxyTheme";
import type { PropsWithTheme } from "@galaxy-io/dls/theme/types";
import Widget from "@galaxy-io/dls/widget/Widget";

import { GetPipelineRequestSchema } from "@/gen/ingestion/v1/pipelines_pb";

import BaseHeader, { BaseHeaderSize } from "@/layouts/components/BaseHeader";

import PipelineSettingsPageDanger from "@/pages/pipelines/settings/PipelineSettingsPageDanger";
import PipelineSettingsPageGeneral from "@/pages/pipelines/settings/PipelineSettingsPageGeneral";
import PipelineSettingsPageSchedule from "@/pages/pipelines/settings/PipelineSettingsPageSchedule";

import { useSuspenseGetPipelineQuery } from "@/api/queries/pipelines";

const PageWrapper = withTheme(styled.div<PropsWithTheme>`
  width: 100%;
  height: 100%;

  display: flex;
  flex-direction: column;

  overflow: hidden;

  background-color: ${({ theme }) => theme.color.background.base};
`);

const ScrollWrapper = styled.div`
  width: 100%;
  flex: 1;
  min-height: 0;
  overflow-y: auto;
`;

const PipelineSettingsPage = () => {
  const { id } = useParams({ from: "/pipelines/$id" });

  const { data } = useSuspenseGetPipelineQuery({
    input: create(GetPipelineRequestSchema, { id }),
  });

  if (!data.pipeline) {
    throw new Error(`Pipeline ${id} not found`);
  }

  return (
    <PageWrapper>
      <Wrapper padding={"16px"} fillWidth>
        <BaseHeader size={BaseHeaderSize.LARGE} title="Settings" />
      </Wrapper>
      <HorizontalDivider />
      <ScrollWrapper>
        <FlexWrapper
          direction={FlexDirection.COLUMN}
          gap={16}
          padding={"16px"}
          minWidth={400}
          maxWidth={600}
        >
          <Widget noHover fillWidth>
            <FlexWrapper
              fillWidth
              alignItems={AlignItems.CENTER}
              justifyContent={JustifyContent.SPACE_BETWEEN}
            >
              <Text weight={TextWeight.MEDIUM}>Pipeline ID</Text>
              <CopyInput value={data.pipeline.id} size={InputSize.SMALL} width={272} isMonospace />
            </FlexWrapper>
          </Widget>
          <PipelineSettingsPageGeneral pipeline={data.pipeline} />
          <PipelineSettingsPageSchedule pipelineId={data.pipeline.id} schedule={data.schedule} />
          <PipelineSettingsPageDanger pipeline={data.pipeline} />
        </FlexWrapper>
      </ScrollWrapper>
    </PageWrapper>
  );
};

export default PipelineSettingsPage;
