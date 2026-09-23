import { useMemo } from "react";

import { create } from "@bufbuild/protobuf";
import { keepPreviousData } from "@tanstack/react-query";

import { useDebouncedValue } from "@galaxy-io/dls/inputs/hooks";

import type { ResourceColumn } from "@/gen/ingestion/v1/connectors_pb";
import { ValidateTransformRequestSchema } from "@/gen/ingestion/v1/transformations_pb";

import { TRANSFORM_VALIDATION_DEBOUNCE_MS } from "@/pages/pipelines/components/transform/constants";
import {
  getTransformStepIssues,
  getTransformStepPath,
  groupTransformIssuesByStep,
  mapTransformExpressionTypes,
} from "@/pages/pipelines/components/transform/grammar/paths";
import { serializeTransformDefinition } from "@/pages/pipelines/components/transform/grammar/serialize";
import { usePipelineTransformFieldsIssues } from "@/pages/pipelines/components/transform/hooks/usePipelineTransformFieldsIssues";
import {
  usePipelineTransformFieldsEnvironment,
  usePipelineTransformFieldsState,
} from "@/pages/pipelines/components/transform/PipelineTransformFieldsProvider";
import type {
  PipelineTransformFieldsDraft,
  PipelineTransformFieldsEditor,
} from "@/pages/pipelines/components/transform/types";
import {
  getTransformTypeSummary,
  placeTransformDraft,
} from "@/pages/pipelines/components/transform/utils";
import {
  getTransformDraftError,
  isTransformDraftComplete,
  isTransformDraftValidatable,
} from "@/pages/pipelines/components/transform/validation";

import { useValidateTransformQuery } from "@/api/queries/transforms";

const NO_COLUMNS: ResourceColumn[] = [];
const NO_ERRORS = new Map<string, string[]>();
const NO_TYPES = new Map<string, string>();

export const usePipelineTransformFieldsValidation = (draft: PipelineTransformFieldsDraft) => {
  const { stepsByResource } = usePipelineTransformFieldsState();
  const { columnsByResource, sourceConnectionId, functionsByName, grammarVersion, isReadOnly } =
    usePipelineTransformFieldsEnvironment();
  const savedIssues = usePipelineTransformFieldsIssues(draft.resource);
  const requiresValidation = sourceConnectionId !== "" && draft.resource !== "";

  const prefixDefinition = useMemo(() => {
    if (draft.resource === "") return undefined;
    const placed = placeTransformDraft(stepsByResource, draft);
    const steps = (placed.stepsByResource.get(draft.resource) ?? []).slice(0, placed.index);
    return serializeTransformDefinition(new Map([[draft.resource, steps]]), grammarVersion);
  }, [draft, grammarVersion, stepsByResource]);
  const hasPrefix = requiresValidation && prefixDefinition !== undefined;
  const prefix = useValidateTransformQuery({
    input: create(ValidateTransformRequestSchema, {
      sourceConnectionId,
      resource: draft.resource,
      transform: prefixDefinition,
    }),
    options: { enabled: hasPrefix },
  });
  const columns = hasPrefix
    ? (prefix.data?.outputColumns ?? NO_COLUMNS)
    : (columnsByResource.get(draft.resource) ?? NO_COLUMNS);
  const prefixError =
    prefix.error !== null ? "Unable to load the columns available before this step." : null;
  const isPrefixPending = hasPrefix && prefix.data === undefined && prefixError === null;

  const isComplete = isTransformDraftComplete(draft, columns, functionsByName);
  const isValidatable = isTransformDraftValidatable(draft);
  const clientError = getTransformDraftError(draft, functionsByName);

  const debounced = useDebouncedValue(draft, TRANSFORM_VALIDATION_DEBOUNCE_MS);
  const isSettled = debounced === draft;
  const placed = useMemo(
    () => placeTransformDraft(stepsByResource, debounced),
    [debounced, stepsByResource],
  );
  const definition = useMemo(
    () => serializeTransformDefinition(placed.stepsByResource, grammarVersion),
    [placed, grammarVersion],
  );
  const validation = useValidateTransformQuery({
    input: create(ValidateTransformRequestSchema, {
      sourceConnectionId,
      resource: debounced.resource,
      transform: definition,
    }),
    options: {
      enabled:
        requiresValidation &&
        isValidatable &&
        isSettled &&
        !isPrefixPending &&
        prefixError === null,
      placeholderData: keepPreviousData,
    },
  });
  const stepPath = getTransformStepPath(debounced.resource, placed.index);
  const isFresh = isSettled && validation.data !== undefined && !validation.isPlaceholderData;
  const showsIssues = isFresh && isComplete && clientError === null;
  const validationError =
    isSettled && validation.error !== null ? "Unable to validate this transformation." : null;

  const { errors, downstream } = useMemo(() => {
    if (!showsIssues || !validation.data) return { errors: NO_ERRORS, downstream: [] };
    const steps = placed.stepsByResource.get(debounced.resource) ?? [];
    const downstream = [...groupTransformIssuesByStep(validation.data.issues)].flatMap(
      ([index, messages]) => {
        const step = steps[index];
        if (!step || index <= placed.index) return [];
        const saved = savedIssues.get(step.id) ?? [];
        return messages
          .filter((message) => !saved.includes(message))
          .map((message) => `Step ${index + 1}: ${message}`);
      },
    );
    return { errors: getTransformStepIssues(validation.data.issues, stepPath), downstream };
  }, [debounced.resource, placed, savedIssues, showsIssues, stepPath, validation.data]);
  const types = useMemo(
    () =>
      validation.data
        ? mapTransformExpressionTypes(validation.data.expressionTypes, stepPath)
        : NO_TYPES,
    [stepPath, validation.data],
  );

  const isDisabled = isReadOnly || isPrefixPending || prefixError !== null || draft.resource === "";
  const editor = useMemo<PipelineTransformFieldsEditor>(
    () => ({ columns, types, errors, isDisabled }),
    [columns, types, errors, isDisabled],
  );
  const { typeSummary, warning } = getTransformTypeSummary(draft, editor, functionsByName);

  const ownMessages = [...errors.values()].flat();
  const issues = [prefixError, clientError, validationError, ...ownMessages].filter(
    (message) => message !== null,
  );
  const warnings = [warning, ...downstream].filter((message) => message !== null);
  const isSaveDisabled =
    isReadOnly ||
    !isComplete ||
    issues.length > 0 ||
    isPrefixPending ||
    (requiresValidation && !isFresh);

  return { editor, issues, warnings, typeSummary, isSaveDisabled };
};
