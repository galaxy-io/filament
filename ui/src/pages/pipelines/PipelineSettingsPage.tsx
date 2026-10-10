import type { FC } from "react";

import { notFound } from "@tanstack/react-router";

import Flex, { AlignItems, FlexDirection } from "@galaxy-io/dls/layout/Flex";
import ScrollArea from "@galaxy-io/dls/layout/ScrollArea";

import PipelineSettingsPageAdvanced from "@/pages/pipelines/settings/PipelineSettingsPageAdvanced";
import PipelineSettingsPageDanger from "@/pages/pipelines/settings/PipelineSettingsPageDanger";
import PipelineSettingsPageGeneral from "@/pages/pipelines/settings/PipelineSettingsPageGeneral";
import PipelineSettingsPageNotifications from "@/pages/pipelines/settings/PipelineSettingsPageNotifications";
import PipelineSettingsPageSchedule from "@/pages/pipelines/settings/PipelineSettingsPageSchedule";

import { usePipelineParams } from "@/module/hooks";

import { createGetPipelineInput, useSuspenseGetPipelineQuery } from "@/api/queries/pipelines";

const PipelineSettingsPage: FC = () => {
  const { id } = usePipelineParams();

  const { data } = useSuspenseGetPipelineQuery({
    input: createGetPipelineInput(id),
  });

  if (!data.pipeline) {
    throw notFound();
  }

  return (
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
        <PipelineSettingsPageNotifications />
        <PipelineSettingsPageAdvanced />
        <PipelineSettingsPageDanger />
      </Flex>
    </ScrollArea>
  );
};

export default PipelineSettingsPage;
