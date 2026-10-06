import Beacon from "@galaxy-io/dls/beacons/Beacon";
import CodeEditor, { CodeEditorLanguage } from "@galaxy-io/dls/editor/CodeEditor";
import { InputSize } from "@galaxy-io/dls/inputs/Input";
import MultiSelectInput, { MultiSelectInputSize } from "@galaxy-io/dls/inputs/MultiSelectInput";
import PasswordInput from "@galaxy-io/dls/inputs/PasswordInput";
import SelectInput, { SelectInputSize, type SelectOption } from "@galaxy-io/dls/inputs/SelectInput";
import TextInput from "@galaxy-io/dls/inputs/TextInput";
import Flex, { AlignItems, FlexDirection } from "@galaxy-io/dls/layout/Flex";
import Text, { TextSize, TextVariant } from "@galaxy-io/dls/text/Text";

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
import { PIPELINE_RUN_STATUS_TO_BEACON_VARIANT_MAP } from "@/pages/pipelines/history/constants";

const EVENT_OPTIONS: SelectOption[] = PIPELINE_NOTIFIER_EVENT_OPTIONS.map((option) => ({
  ...option,
  icon: (
    <Beacon
      variant={
        PIPELINE_RUN_STATUS_TO_BEACON_VARIANT_MAP[
          PIPELINE_NOTIFIER_EVENT_TO_RUN_STATUS_MAP[option.value as NotifierEvent]
        ]
      }
    />
  ),
}));

interface PipelineNotifierFieldsProps {
  state: PipelineNotifierState;
  onChange: (partial: Partial<PipelineNotifierState>) => void;
  isDisabled?: boolean;
}

const PipelineNotifierFields = ({
  state,
  onChange,
  isDisabled = false,
}: PipelineNotifierFieldsProps) => {
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
  const selectedTypeOption =
    PIPELINE_NOTIFIER_TYPE_OPTIONS.find((option) => option.value === state.notificationType) ??
    null;
  const selectedEventOptions = EVENT_OPTIONS.filter((option) =>
    state.events.includes(option.value as NotifierEvent),
  );

  const handleNameChange = (name: string) => {
    onChange({ name });
  };

  const handleTypeChange = (option: SelectOption) => {
    onChange({ notificationType: option.value as NotificationType, url: "", headers: "" });
  };

  const handleEventsChange = (options: SelectOption[]) => {
    onChange({ events: options.map((option) => option.value as NotifierEvent) });
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
        size={InputSize.LARGE}
        isDisabled={isDisabled}
        fillWidth
      />
      <SelectInput
        label="Type"
        options={PIPELINE_NOTIFIER_TYPE_OPTIONS}
        /* @dls-migrate selectinput.value: `value` and `onChange` now carry option ids, not option objects. */ value={
          selectedTypeOption
        }
        onChange={handleTypeChange}
        placeholder="Select type"
        size={SelectInputSize.LARGE}
        isDisabled={isDisabled}
        fillWidth
      />
      <MultiSelectInput
        label="Events"
        options={EVENT_OPTIONS}
        /* @dls-migrate multiselectinput.value: `value` and `onChange` now carry option ids, not option objects. */ value={
          selectedEventOptions
        }
        onChange={handleEventsChange}
        /* @dls-migrate multiselectinput.renderSelectedText: Merged into `renderValue(options)`. */ renderSelectedText={
          formatPipelineNotifierEventsSelection
        }
        /* @dls-migrate multiselectinput.pinnedOptions: Pinned rows are now option ids: pass `pinnedIds`. */ pinnedOptions={[
          PIPELINE_NOTIFIER_ALL_EVENTS_PINNED_OPTION,
        ]}
        placeholder="Select events"
        size={MultiSelectInputSize.LARGE}
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
          size={InputSize.LARGE}
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
            size={InputSize.LARGE}
            isDisabled={isDisabled}
            fillWidth
          />
          <Flex alignItems={AlignItems.START} direction={FlexDirection.COLUMN} gap={8} fillWidth>
            <Text size={TextSize.BODY_SM} variant={TextVariant.SECONDARY}>
              Headers
            </Text>
            <CodeEditor
              value={state.headers}
              onChange={handleHeadersChange}
              language={CodeEditorLanguage.JSON}
              placeholder={headersPlaceholder}
              /* @dls-migrate codeeditor.isReadOnly: A read-only editor used to show code is a `CodeBlock` (or `JsonViewer` for JSON). */ isReadOnly={
                isDisabled
              }
              hasLineNumbers={false}
            />
            {headersError && (
              <Text size={TextSize.BODY_SM} variant={TextVariant.ERROR}>
                {headersError}
              </Text>
            )}
          </Flex>
        </>
      )}
    </Flex>
  );
};

export default PipelineNotifierFields;
