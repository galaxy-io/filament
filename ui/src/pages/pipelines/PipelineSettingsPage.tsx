import type { FC } from "react";

import { create } from "@bufbuild/protobuf";
import { notFound, useParams } from "@tanstack/react-router";

import Box, { BoxVariant } from "@galaxy-io/dls/layout/Box";
import Divider from "@galaxy-io/dls/layout/Divider";
import Flex, { AlignItems, FlexDirection } from "@galaxy-io/dls/layout/Flex";
import FlexItem from "@galaxy-io/dls/layout/FlexItem";
import ScrollArea from "@galaxy-io/dls/layout/ScrollArea";

import { GetPipelineRequestSchema } from "@/gen/ingestion/v1/pipelines_pb";

import BaseHeader, { BaseHeaderSize } from "@/layouts/components/BaseHeader";

import PipelineSettingsPageAdvanced from "@/pages/pipelines/settings/PipelineSettingsPageAdvanced";
import PipelineSettingsPageDanger from "@/pages/pipelines/settings/PipelineSettingsPageDanger";
import PipelineSettingsPageGeneral from "@/pages/pipelines/settings/PipelineSettingsPageGeneral";
import PipelineSettingsPageNotifications from "@/pages/pipelines/settings/PipelineSettingsPageNotifications";
import PipelineSettingsPageSchedule from "@/pages/pipelines/settings/PipelineSettingsPageSchedule";

import { useSuspenseGetPipelineQuery } from "@/api/queries/pipelines";

const PipelineSettingsPage: FC = () => {
  const { id } = useParams({ from: "/_app/pipelines/$id" });

  const { data } = useSuspenseGetPipelineQuery({
    input: create(GetPipelineRequestSchema, { id }),
  });

  if (!data.pipeline) {
    throw notFound();
  }

  return (
    <Box variant={BoxVariant.BASE} fillWidth height="100%" overflow="hidden">
      <Flex direction={FlexDirection.COLUMN} height="100%">
        <Box padding={16} fillWidth>
          <BaseHeader size={BaseHeaderSize.LARGE} title="Settings" />
        </Box>
        <Divider />
        <FlexItem grow={1} basis={0} minHeight={0} fillWidth>
          <ScrollArea>
            <Flex
              alignItems={AlignItems.STRETCH}
              direction={FlexDirection.COLUMN}
              gap={12}
              padding={16}
              minWidth={400}
              maxWidth={640}
            >
              <PipelineSettingsPageGeneral />
              <PipelineSettingsPageSchedule />
              <PipelineSettingsPageNotifications pipeline={data.pipeline} />
              <PipelineSettingsPageAdvanced />
              <PipelineSettingsPageDanger />
            </Flex>
          </ScrollArea>
        </FlexItem>
      </Flex>
    </Box>
  );
};

export default PipelineSettingsPage;
