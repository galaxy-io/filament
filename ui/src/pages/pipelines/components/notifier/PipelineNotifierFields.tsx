import type { FC } from "react";

import CodeEditor, { CodeEditorLanguage } from "@galaxy-io/dls/editor/CodeEditor";
import Field from "@galaxy-io/dls/inputs/Field";
import MultiSelectInput from "@galaxy-io/dls/inputs/MultiSelectInput";
import PasswordInput from "@galaxy-io/dls/inputs/PasswordInput";
import SelectInput, { type SelectOption } from "@galaxy-io/dls/inputs/SelectInput";
import TextInput from "@galaxy-io/dls/inputs/TextInput";
import Flex, { AlignItems, FlexDirection } from "@galaxy-io/dls/layout/Flex";
import Text from "@galaxy-io/dls/text/Text";

import { NotificationType, type NotifierEvent } from "@/gen/ingestion/v1/notifiers_pb";

import {
  PIPELINE_NOTIFIER_ALL_EVENTS_PINNED_OPTION,
  PIPELINE_NOTIFIER_EVENT_OPTIONS,
  PIPELINE_NOTIFIER_EVENT_TO_RUN_STATUS_MAP,
  PIPELINE_NOTIFIER_HEADERS_KEEP_PLACEHOLDER_TEXT,
  PIPELINE_NOTIFIER_HEADERS_PLACEHOLDER_TEXT,
  PIPELINE_NOTIFIER_HEADERS_SECRET_REF_KEY,
  PIPELINE_NOTIFIER_TYPE_OPTIONS,
} from "@/pages/pipelines/components/notifier/constants";
import type { PipelineNotifierState } from "@/pages/pipelines/components/notifier/types";
import {
  formatPipelineNotifierEventsSelection,
  hasPipelineNotifierStoredUrl,
  isPipelineNotifierSlackUrlValid,
  isPipelineNotifierUrlValid,
  parsePipelineNotifierHeaders,
} from "@/pages/pipelines/components/notifier/utils";
import PipelineRunStatusSwatch from "@/pages/pipelines/history/PipelineRunStatusSwatch";

import { getSelectAllChange, getSelectAllOptions, getSelectAllValue } from "@/utils/select";

const EVENT_OPTIONS: SelectOption[] = getSelectAllOptions(
  PIPELINE_NOTIFIER_ALL_EVENTS_PINNED_OPTION,
  PIPELINE_NOTIFIER_EVENT_OPTIONS.map((option) => ({
    ...option,
    leading: (
      <PipelineRunStatusSwatch
        status={PIPELINE_NOTIFIER_EVENT_TO_RUN_STATUS_MAP[Number(option.id) as NotifierEvent]}
      />
    ),
  })),
);

interface PipelineNotifierFieldsProps {
  state: PipelineNotifierState;
  onChange: (partial: Partial<PipelineNotifierState>) => void;
  isDisabled?: boolean;
}

const PipelineNotifierFields: FC<PipelineNotifierFieldsProps> = ({
  state,
  onChange,
  isDisabled = false,
}) => {
  const isSlack = state.notificationType === NotificationType.SLACK;
  const urlError =
    state.url !== "" && !isPipelineNotifierUrlValid(state.url)
      ? "Use an absolute http or https URL"
      : undefined;
  const slackUrlError =
    state.url !== "" && !isPipelineNotifierSlackUrlValid(state.url)
      ? "Use a Slack incoming webhook URL"
      : undefined;
  const slackUrlPlaceholder = hasPipelineNotifierStoredUrl(state)
    ? "Leave blank to keep current value"
    : "https://hooks.slack.com/services/...";
  const headersError =
    parsePipelineNotifierHeaders(state.headers) === null
      ? "Use a JSON object with string values"
      : undefined;
  const headersPlaceholder =
    PIPELINE_NOTIFIER_HEADERS_SECRET_REF_KEY in state.secretRefs
      ? PIPELINE_NOTIFIER_HEADERS_KEEP_PLACEHOLDER_TEXT
      : PIPELINE_NOTIFIER_HEADERS_PLACEHOLDER_TEXT;
  const selectedEventIds = state.events.map(String);

  const handleNameChange = (name: string) => {
    onChange({ name });
  };

  const handleTypeChange = (id: string | null) => {
    if (id === null) return;
    onChange({ notificationType: Number(id) as NotificationType, url: "", headers: "" });
  };

  const handleEventsChange = (ids: string[]) => {
    const next = getSelectAllChange(
      PIPELINE_NOTIFIER_ALL_EVENTS_PINNED_OPTION,
      ids,
      selectedEventIds,
    );
    onChange({ events: next.map((id) => Number(id) as NotifierEvent) });
  };

  const handleUrlChange = (url: string) => {
    onChange({ url });
  };

  const handleHeadersChange = (headers: string) => {
    onChange({ headers });
  };

  return (
    <Flex alignItems={AlignItems.START} direction={FlexDirection.COLUMN} gap={12} fillWidth>
      <TextInput
        label="Name"
        value={state.name}
        onChange={handleNameChange}
        placeholder="Notifier name"
        isDisabled={isDisabled}
        fillWidth
      />
      <SelectInput
        label="Type"
        options={PIPELINE_NOTIFIER_TYPE_OPTIONS}
        value={String(state.notificationType)}
        onChange={handleTypeChange}
        placeholder="Select type"
        isDisabled={isDisabled}
        fillWidth
      />
      <MultiSelectInput
        label="Events"
        options={EVENT_OPTIONS}
        value={getSelectAllValue(PIPELINE_NOTIFIER_ALL_EVENTS_PINNED_OPTION, selectedEventIds)}
        onChange={handleEventsChange}
        renderValue={(options) => <Text>{formatPipelineNotifierEventsSelection(options)}</Text>}
        pinnedIds={[PIPELINE_NOTIFIER_ALL_EVENTS_PINNED_OPTION.id]}
        placeholder="Select events"
        isDisabled={isDisabled}
        fillWidth
      />
      {isSlack ? (
        <PasswordInput
          label="Webhook URL"
          value={state.url}
          onChange={handleUrlChange}
          error={slackUrlError}
          placeholder={slackUrlPlaceholder}
          isDisabled={isDisabled}
          fillWidth
        />
      ) : (
        <>
          <TextInput
            label="URL"
            value={state.url}
            onChange={handleUrlChange}
            error={urlError}
            placeholder="https://example.com/hooks/filament"
            isDisabled={isDisabled}
            fillWidth
          />
          <Field label="Headers" error={headersError} fillWidth>
            <CodeEditor
              value={state.headers}
              onChange={handleHeadersChange}
              language={CodeEditorLanguage.JSON}
              placeholder={headersPlaceholder}
              isReadOnly={isDisabled}
              hasLineNumbers={false}
            />
          </Field>
        </>
      )}
    </Flex>
  );
};

export default PipelineNotifierFields;
