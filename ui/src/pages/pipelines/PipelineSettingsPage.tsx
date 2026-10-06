import { create } from "@bufbuild/protobuf";
import { styled } from "@linaria/react";
import { notFound, useParams } from "@tanstack/react-router";

import Box from "@galaxy-io/dls/layout/Box";
import Divider from "@galaxy-io/dls/layout/Divider";
import Flex, { AlignItems, FlexDirection } from "@galaxy-io/dls/layout/Flex";
import { t } from "@galaxy-io/dls/theme/tokens/t";

import { GetPipelineRequestSchema } from "@/gen/ingestion/v1/pipelines_pb";

import BaseHeader, { BaseHeaderSize } from "@/layouts/components/BaseHeader";

import PipelineSettingsPageAdvanced from "@/pages/pipelines/settings/PipelineSettingsPageAdvanced";
import PipelineSettingsPageDanger from "@/pages/pipelines/settings/PipelineSettingsPageDanger";
import PipelineSettingsPageGeneral from "@/pages/pipelines/settings/PipelineSettingsPageGeneral";
import PipelineSettingsPageNotifications from "@/pages/pipelines/settings/PipelineSettingsPageNotifications";
import PipelineSettingsPageSchedule from "@/pages/pipelines/settings/PipelineSettingsPageSchedule";

import { useSuspenseGetPipelineQuery } from "@/api/queries/pipelines";

const PageWrapper = styled.div`
  width: 100%;
  height: 100%;

  display: flex;
  flex-direction: column;

  overflow: hidden;

  background-color: ${t.color.background.base};
`;

const ScrollWrapper = styled.div`
  width: 100%;
  flex: 1;
  min-height: 0;
  overflow-y: auto;
`;

const PipelineSettingsPage = () => {
  const { id } = useParams({ from: "/_app/pipelines/$id" });

  const { data } = useSuspenseGetPipelineQuery({
    input: create(GetPipelineRequestSchema, { id }),
  });

  if (!data.pipeline) {
    throw notFound();
  }

  return (
    <PageWrapper>
      <Box padding={16} fillWidth>
        <BaseHeader size={BaseHeaderSize.LARGE} title="Settings" />
      </Box>
      <Divider />
      <ScrollWrapper>
        <Flex
          alignItems={AlignItems.START}
          direction={FlexDirection.COLUMN}
          gap={12}
          padding={16}
          minWidth={400}
          maxWidth={640}
        >
          <PipelineSettingsPageGeneral />
          <PipelineSettingsPageSchedule />
          <PipelineSettingsPageNotifications />
          <PipelineSettingsPageAdvanced />
          <PipelineSettingsPageDanger />
        </Flex>
      </ScrollWrapper>
    </PageWrapper>
  );
};

export default PipelineSettingsPage;
