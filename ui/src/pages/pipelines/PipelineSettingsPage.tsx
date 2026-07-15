import { useState } from "react";

import { TrashIcon } from "@phosphor-icons/react";
import { styled } from "@linaria/react";
import { useParams, useNavigate } from "@tanstack/react-router";

import Button, { ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import FlexWrapper, {
  FlexDirection,
  FlexGap,
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

import { useGetPipelineQuery, useDeletePipelineMutation } from "@/api/queries/pipelines";

const PageWrapper = withTheme(styled.div<PropsWithTheme>`
  width: 100%;
  height: 100%;

  padding: 24px;

  overflow-y: auto;

  background-color: ${({ theme }) => theme.color.background.primary};
`);

const ContentWrapper = styled.div`
  display: flex;
  flex-direction: column;
  gap: 16px;

  max-width: 600px;
`;

const WidgetContent = styled.div`
  display: flex;
  flex-direction: column;
  gap: 16px;
`;

const FieldWrapper = styled.div`
  display: flex;
  flex-direction: column;
  gap: 6px;
`;

const FieldRow = styled.div`
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
`;

const PipelineSettingsPage = () => {
  const { id } = useParams({ from: "/pipelines/$id" });
  const navigate = useNavigate();
  const { data } = useGetPipelineQuery({ input: { id } });
  const deletePipeline = useDeletePipelineMutation();

  const pipeline = data?.pipeline;

  const [description, setDescription] = useState("");
  const [scheduleEnabled, setScheduleEnabled] = useState(false);
  const [notificationsEnabled, setNotificationsEnabled] = useState(true);
  const [errorAlertsEnabled, setErrorAlertsEnabled] = useState(true);

  return (
    <PageWrapper>
      <ContentWrapper>
        <FlexWrapper direction={FlexDirection.COLUMN} gap={FlexGap.SMALL}>
          <Text size={TextSize.HEADING_SM} weight={TextWeight.MEDIUM}>
            Settings
          </Text>
          <Text size={TextSize.BODY_SM} variant={TextVariant.SECONDARY}>
            Configure your pipeline settings and preferences.
          </Text>
        </FlexWrapper>

        <Widget header="General" noHover>
          <WidgetContent>
            <FieldWrapper>
              <Text size={TextSize.BODY_SM} variant={TextVariant.SECONDARY}>
                Name
              </Text>
              <TextInput
                value={pipeline?.name ?? ""}
                onChange={() => {}}
                placeholder="Pipeline name"
                fillWidth
                isDisabled
              />
            </FieldWrapper>

            <FieldWrapper>
              <Text size={TextSize.BODY_SM} variant={TextVariant.SECONDARY}>
                Description
              </Text>
              <TextAreaInput
                value={description}
                onChange={setDescription}
                placeholder="Optional description"
                fillWidth
              />
            </FieldWrapper>
          </WidgetContent>
        </Widget>

        <Widget header="Schedule" noHover>
          <FieldRow>
            <FlexWrapper direction={FlexDirection.COLUMN} gap={FlexGap.XSMALL}>
              <Text>Enable scheduling</Text>
              <Text size={TextSize.BODY_SM} variant={TextVariant.SECONDARY}>
                Run this pipeline on a recurring schedule.
              </Text>
            </FlexWrapper>
            <ToggleInput
              value={scheduleEnabled}
              onChange={setScheduleEnabled}
            />
          </FieldRow>
        </Widget>

        <Widget header="Notifications" noHover>
          <WidgetContent>
            <FieldRow>
              <FlexWrapper direction={FlexDirection.COLUMN} gap={FlexGap.XSMALL}>
                <Text>Pipeline notifications</Text>
                <Text size={TextSize.BODY_SM} variant={TextVariant.SECONDARY}>
                  Receive notifications when runs complete.
                </Text>
              </FlexWrapper>
              <ToggleInput
                value={notificationsEnabled}
                onChange={setNotificationsEnabled}
              />
            </FieldRow>

            <FieldRow>
              <FlexWrapper direction={FlexDirection.COLUMN} gap={FlexGap.XSMALL}>
                <Text>Error alerts</Text>
                <Text size={TextSize.BODY_SM} variant={TextVariant.SECONDARY}>
                  Get notified immediately when errors occur.
                </Text>
              </FlexWrapper>
              <ToggleInput
                value={errorAlertsEnabled}
                onChange={setErrorAlertsEnabled}
              />
            </FieldRow>
          </WidgetContent>
        </Widget>

        <Widget header="Danger zone" variant={WidgetVariant.ERROR} noHover>
          <FieldRow>
            <FlexWrapper direction={FlexDirection.COLUMN} gap={FlexGap.XSMALL}>
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
                deletePipeline.mutate(
                  { id },
                  {
                    onSuccess: () => {
                      navigate({ to: "/pipelines" });
                    },
                  }
                );
              }}
            />
          </FieldRow>
        </Widget>
      </ContentWrapper>
    </PageWrapper>
  );
};

export default PipelineSettingsPage;
