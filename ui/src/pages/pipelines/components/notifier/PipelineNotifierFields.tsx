import FlexWrapper, {
  AlignItems,
  FlexDirection,
  FlexGap,
  JustifyContent,
} from "@galaxy-io/dls/containers/FlexWrapper";
import CodeEditor from "@galaxy-io/dls/editor/CodeEditor";
import { InputSize } from "@galaxy-io/dls/inputs/Input";
import MultiSelectInput from "@galaxy-io/dls/inputs/MultiSelectInput";
import SelectInput, { type SelectInputOption } from "@galaxy-io/dls/inputs/SelectInput";
import TextInput from "@galaxy-io/dls/inputs/TextInput";
import Text, { TextSize, TextVariant } from "@galaxy-io/dls/text/Text";

import type { NotificationType, NotifierEvent } from "@/gen/ingestion/v1/notifiers_pb";

import {
  PIPELINE_NOTIFIER_ALL_EVENTS_PINNED_OPTION,
  PIPELINE_NOTIFIER_EVENT_OPTIONS,
  PIPELINE_NOTIFIER_HEADERS_KEEP_PLACEHOLDER_TEXT,
  PIPELINE_NOTIFIER_HEADERS_PLACEHOLDER_TEXT,
  PIPELINE_NOTIFIER_HEADERS_SECRET_REF_KEY,
  PIPELINE_NOTIFIER_TYPE_OPTIONS,
} from "@/pages/pipelines/components/notifier/constants";
import type { PipelineNotifierState } from "@/pages/pipelines/components/notifier/types";
import {
  formatPipelineNotifierEventsSelection,
  isPipelineNotifierUrlValid,
  parsePipelineNotifierHeaders,
} from "@/pages/pipelines/components/notifier/utils";
import { PIPELINE_SETTINGS_INPUT_WIDTH } from "@/pages/pipelines/settings/constants";

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
  const urlError =
    state.url !== "" && !isPipelineNotifierUrlValid(state.url)
      ? "Use an absolute http or https URL"
      : undefined;
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
  const selectedEventOptions = PIPELINE_NOTIFIER_EVENT_OPTIONS.filter((option) =>
    state.events.includes(option.value as NotifierEvent),
  );

  const handleNameChange = (name: string) => {
    onChange({ name });
  };

  const handleTypeChange = (option: SelectInputOption) => {
    onChange({ notificationType: option.value as NotificationType });
  };

  const handleEventsChange = (options: SelectInputOption[]) => {
    onChange({ events: options.map((option) => option.value as NotifierEvent) });
  };

  const handleUrlChange = (url: string) => {
    onChange({ url });
  };

  const handleHeadersChange = (headers: string) => {
    onChange({ headers });
  };

  return (
    <FlexWrapper direction={FlexDirection.COLUMN} gap={FlexGap.MEDIUM} fillWidth>
      <FlexWrapper
        alignItems={AlignItems.CENTER}
        justifyContent={JustifyContent.SPACE_BETWEEN}
        fillWidth
      >
        <Text variant={TextVariant.SECONDARY}>Name</Text>
        <TextInput
          value={state.name}
          onChange={handleNameChange}
          placeholder="Webhook name"
          size={InputSize.LARGE}
          width={PIPELINE_SETTINGS_INPUT_WIDTH}
          isDisabled={isDisabled}
        />
      </FlexWrapper>
      <FlexWrapper
        alignItems={AlignItems.CENTER}
        justifyContent={JustifyContent.SPACE_BETWEEN}
        fillWidth
      >
        <Text variant={TextVariant.SECONDARY}>Type</Text>
        <SelectInput
          options={PIPELINE_NOTIFIER_TYPE_OPTIONS}
          value={selectedTypeOption}
          onChange={handleTypeChange}
          placeholder="Select type"
          size={InputSize.LARGE}
          width={PIPELINE_SETTINGS_INPUT_WIDTH}
          isDisabled={isDisabled}
        />
      </FlexWrapper>
      <FlexWrapper
        alignItems={AlignItems.CENTER}
        justifyContent={JustifyContent.SPACE_BETWEEN}
        fillWidth
      >
        <Text variant={TextVariant.SECONDARY}>Events</Text>
        <MultiSelectInput
          options={PIPELINE_NOTIFIER_EVENT_OPTIONS}
          value={selectedEventOptions}
          onChange={handleEventsChange}
          renderSelectedText={formatPipelineNotifierEventsSelection}
          pinnedOptions={[PIPELINE_NOTIFIER_ALL_EVENTS_PINNED_OPTION]}
          placeholder="Select events"
          size={InputSize.LARGE}
          width={PIPELINE_SETTINGS_INPUT_WIDTH}
          isDisabled={isDisabled}
        />
      </FlexWrapper>
      <FlexWrapper
        alignItems={AlignItems.CENTER}
        justifyContent={JustifyContent.SPACE_BETWEEN}
        fillWidth
      >
        <Text variant={TextVariant.SECONDARY}>URL</Text>
        <TextInput
          value={state.url}
          onChange={handleUrlChange}
          error={urlError}
          placeholder="https://example.com/hooks/filament"
          size={InputSize.LARGE}
          width={PIPELINE_SETTINGS_INPUT_WIDTH}
          isDisabled={isDisabled}
        />
      </FlexWrapper>
      <FlexWrapper
        alignItems={AlignItems.START}
        justifyContent={JustifyContent.SPACE_BETWEEN}
        fillWidth
      >
        <Text variant={TextVariant.SECONDARY}>Headers</Text>
        <FlexWrapper direction={FlexDirection.COLUMN} gap={4} width={PIPELINE_SETTINGS_INPUT_WIDTH}>
          <CodeEditor
            content={state.headers}
            onChange={handleHeadersChange}
            lang="json"
            placeholder={headersPlaceholder}
            borderRadius={4}
            isReadOnly={isDisabled}
            noLineNumbers
          />
          {headersError && (
            <Text size={TextSize.BODY_SM} variant={TextVariant.ERROR}>
              {headersError}
            </Text>
          )}
        </FlexWrapper>
      </FlexWrapper>
    </FlexWrapper>
  );
};

export default PipelineNotifierFields;
