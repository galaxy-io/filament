import { useState } from "react";

import { styled } from "@linaria/react";

import Button, { ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import FlexWrapper, {
  FlexDirection,
  FlexGap,
} from "@galaxy-io/dls/containers/FlexWrapper";
import HorizontalDivider from "@galaxy-io/dls/dividers/HorizontalDivider";
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

const PageWrapper = withTheme(styled.div<PropsWithTheme>`
  width: 100%;
  height: 100%;

  padding: 24px;

  overflow-y: auto;

  background-color: ${({ theme }) => theme.color.background.primary};
`);

const ContentWrapper = styled.div`
  max-width: 600px;
`;

const Section = styled.div`
  display: flex;
  flex-direction: column;
  gap: 16px;

  padding: 20px 0;
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

const DangerSection = withTheme(styled.div<PropsWithTheme>`
  display: flex;
  flex-direction: column;
  gap: 12px;

  padding: 16px;

  border: 1px solid ${({ theme }) => theme.color.border.error};
  border-radius: 8px;
`);

const PipelineSettingsPage = () => {
  const [name, setName] = useState("My Pipeline");
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

        <Section>
          <Text size={TextSize.BODY_MD} weight={TextWeight.MEDIUM}>
            General
          </Text>

          <FieldWrapper>
            <Text size={TextSize.BODY_SM} variant={TextVariant.SECONDARY}>
              Name
            </Text>
            <TextInput
              value={name}
              onChange={setName}
              placeholder="Pipeline name"
              fillWidth
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
        </Section>

        <HorizontalDivider />

        <Section>
          <Text size={TextSize.BODY_MD} weight={TextWeight.MEDIUM}>
            Schedule
          </Text>

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
        </Section>

        <HorizontalDivider />

        <Section>
          <Text size={TextSize.BODY_MD} weight={TextWeight.MEDIUM}>
            Notifications
          </Text>

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
        </Section>

        <HorizontalDivider />

        <Section>
          <Text size={TextSize.BODY_MD} weight={TextWeight.MEDIUM}>
            Danger zone
          </Text>

          <DangerSection>
            <FlexWrapper direction={FlexDirection.COLUMN} gap={FlexGap.XSMALL}>
              <Text weight={TextWeight.MEDIUM}>Delete pipeline</Text>
              <Text size={TextSize.BODY_SM} variant={TextVariant.SECONDARY}>
                Permanently delete this pipeline and all associated data. This
                action cannot be undone.
              </Text>
            </FlexWrapper>
            <div>
              <Button
                label="Delete pipeline"
                variant={ButtonVariant.ERROR}
                onClick={() => {
                  // TODO: Implement delete confirmation
                }}
              />
            </div>
          </DangerSection>
        </Section>
      </ContentWrapper>
    </PageWrapper>
  );
};

export default PipelineSettingsPage;
