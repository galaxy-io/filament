import { useEffect, useMemo, useState } from "react";

import { create } from "@bufbuild/protobuf";
import { styled } from "@linaria/react";
import { TrashIcon } from "@phosphor-icons/react";
import { useNavigate, useParams } from "@tanstack/react-router";

import Button, { ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import FlexWrapper, {
  AlignItems,
  FlexDirection,
  FlexGap,
  JustifyContent,
} from "@galaxy-io/dls/containers/FlexWrapper";
import TextAreaInput from "@galaxy-io/dls/inputs/TextAreaInput";
import TextInput from "@galaxy-io/dls/inputs/TextInput";
import ToggleInput from "@galaxy-io/dls/inputs/ToggleInput";
import Text, {
  TextSize,
  TextVariant,
  TextWeight,
} from "@galaxy-io/dls/text/Text";
import { withTheme } from "@galaxy-io/dls/theme/GalaxyTheme";
import type { PropsWithTheme } from "@galaxy-io/dls/theme/types";
import Widget, { WidgetVariant } from "@galaxy-io/dls/widget/Widget";

import { ToastVariant } from "@/providers/toast/Toast";
import { useToast } from "@/providers/toast/useToast";

import {
  useDeletePipelineMutation,
  useGetPipelineQuery,
} from "@/api/queries/pipelines";

import {
  DeletePipelineRequestSchema,
  GetPipelineRequestSchema,
} from "@/gen/ingestion/v1/pipelines_pb";

const PageWrapper = withTheme(styled.div<PropsWithTheme>`
  width: 100%;
  height: 100%;

  padding: 24px;

  overflow-y: auto;

  background-color: ${({ theme }) => theme.color.background.primary};
`);

interface PipelineSettingsPageState {
  name: string;
  description: string;
  scheduleEnabled: boolean;
  notificationsEnabled: boolean;
  errorAlertsEnabled: boolean;
}

const DEFAULT_STATE: PipelineSettingsPageState = {
  name: "",
  description: "",
  scheduleEnabled: false,
  notificationsEnabled: true,
  errorAlertsEnabled: true,
};

const PipelineSettingsPage = () => {
  const { id } = useParams({ from: "/pipelines/$id" });

  const navigate = useNavigate();

  const { data } = useGetPipelineQuery({
    input: create(GetPipelineRequestSchema, { id }),
  });
  const { mutate: deletePipeline } = useDeletePipelineMutation();

  const { showToast } = useToast();

  const [state, setState] = useState<PipelineSettingsPageState>(DEFAULT_STATE);

  useEffect(() => {
    const { pipeline } = data ?? {};
    if (pipeline) {
      setState((prev) => ({ ...prev, name: pipeline.name ?? "" }));
    }
  }, [data]);

  const handleNameChange = (name: string) => {
    setState((prev) => ({ ...prev, name }));
  };

  const handleDescriptionChange = (description: string) => {
    setState((prev) => ({ ...prev, description }));
  };

  const handleScheduleEnabledChange = (scheduleEnabled: boolean) => {
    setState((prev) => ({ ...prev, scheduleEnabled }));
  };

  const handleNotificationsEnabledChange = (notificationsEnabled: boolean) => {
    setState((prev) => ({ ...prev, notificationsEnabled }));
  };

  const handleErrorAlertsEnabledChange = (errorAlertsEnabled: boolean) => {
    setState((prev) => ({ ...prev, errorAlertsEnabled }));
  };

  const hasChanges = useMemo(() => {
    return state.name !== (data?.pipeline?.name ?? "");
  }, [state.name, data?.pipeline?.name]);

  const handleSave = () => {
    showToast({
      header: "Pipeline saved",
      subheader: "Your pipeline has been saved successfully.",
      variant: ToastVariant.SUCCESS,
    });
  };

  return (
    <PageWrapper>
      <FlexWrapper direction={FlexDirection.COLUMN} gap={16} minWidth={400} maxWidth={600}>
        <FlexWrapper direction={FlexDirection.COLUMN} gap={8} fillWidth>
          <Text size={TextSize.HEADING_SM} weight={TextWeight.MEDIUM}>
            Settings
          </Text>
          <Text size={TextSize.BODY_SM} variant={TextVariant.SECONDARY}>
            Configure your pipeline settings and preferences.
          </Text>
        </FlexWrapper>

        <Widget header="General" noHover fillWidth>
          <FlexWrapper direction={FlexDirection.COLUMN} gap={16} fillWidth>
            <FlexWrapper direction={FlexDirection.COLUMN} gap={6} fillWidth>
              <Text size={TextSize.BODY_SM} variant={TextVariant.SECONDARY}>
                Name
              </Text>
              <TextInput
                value={state.name}
                onChange={handleNameChange}
                placeholder="Pipeline name"
                fillWidth
              />
            </FlexWrapper>

            <FlexWrapper direction={FlexDirection.COLUMN} gap={6} fillWidth>
              <Text size={TextSize.BODY_SM} variant={TextVariant.SECONDARY}>
                Description
              </Text>
              <TextAreaInput
                value={state.description}
                onChange={handleDescriptionChange}
                placeholder="Optional description"
                fillWidth
              />
            </FlexWrapper>

            <FlexWrapper justifyContent={JustifyContent.END} fillWidth>
              <Button
                label="Save"
                isDisabled={!hasChanges}
                onClick={handleSave}
              />
            </FlexWrapper>
          </FlexWrapper>
        </Widget>

        <Widget header="Schedule" noHover fillWidth>
          <FlexWrapper fillWidth alignItems={AlignItems.CENTER} gap={16}>
            <FlexWrapper direction={FlexDirection.COLUMN} gap={6} fillWidth>
              <Text>Enable scheduling</Text>
              <Text size={TextSize.BODY_SM} variant={TextVariant.SECONDARY}>
                Run this pipeline on a recurring schedule.
              </Text>
            </FlexWrapper>
            <ToggleInput
              value={state.scheduleEnabled}
              onChange={handleScheduleEnabledChange}
            />
          </FlexWrapper>
        </Widget>

        <Widget header="Notifications" noHover fillWidth>
          <FlexWrapper
            direction={FlexDirection.COLUMN}
            alignItems={AlignItems.CENTER}
            gap={16}
            fillWidth
          >
            <FlexWrapper alignItems={AlignItems.CENTER} fillWidth>
              <FlexWrapper direction={FlexDirection.COLUMN} gap={6} fillWidth>
                <Text>Pipeline notifications</Text>
                <Text size={TextSize.BODY_SM} variant={TextVariant.SECONDARY}>
                  Receive notifications when runs complete.
                </Text>
              </FlexWrapper>
              <ToggleInput
                value={state.notificationsEnabled}
                onChange={handleNotificationsEnabledChange}
              />
            </FlexWrapper>

            <FlexWrapper alignItems={AlignItems.CENTER} fillWidth>
              <FlexWrapper direction={FlexDirection.COLUMN} gap={6} fillWidth>
                <Text>Error alerts</Text>
                <Text size={TextSize.BODY_SM} variant={TextVariant.SECONDARY}>
                  Get notified immediately when errors occur.
                </Text>
              </FlexWrapper>
              <ToggleInput
                value={state.errorAlertsEnabled}
                onChange={handleErrorAlertsEnabledChange}
              />
            </FlexWrapper>
          </FlexWrapper>
        </Widget>

        <Widget header="Danger zone" variant={WidgetVariant.ERROR} noHover fillWidth>
          <FlexWrapper direction={FlexDirection.COLUMN} gap={16} fillWidth>
            <FlexWrapper direction={FlexDirection.COLUMN} gap={6} fillWidth>
              <Text weight={TextWeight.MEDIUM}>Delete pipeline</Text>
              <Text size={TextSize.BODY_SM} variant={TextVariant.SECONDARY}>
                This will permanently delete this pipeline and all of its data.
              </Text>
            </FlexWrapper>
            <Button
              label="Delete pipeline"
              icon={TrashIcon}
              variant={ButtonVariant.ERROR}
              onClick={() => {
                deletePipeline(create(DeletePipelineRequestSchema, { id }), {
                  onSuccess: () => {
                    navigate({ to: "/pipelines" });
                  },
                });
              }}
            />
          </FlexWrapper>
        </Widget>
      </FlexWrapper>
    </PageWrapper>
  );
};

export default PipelineSettingsPage;
