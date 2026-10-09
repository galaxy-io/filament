import { Fragment, type ReactElement } from "react";

import { styled } from "@linaria/react";
import { match } from "ts-pattern";

import Chip, { ChipSize, ChipVariant } from "@galaxy-io/dls/chips/Chip";
import Span, { SpanVariant } from "@galaxy-io/dls/text/Span";
import { TextWeight } from "@galaxy-io/dls/text/Text";
import { FontFamily } from "@galaxy-io/dls/theme/enums";
import { t } from "@galaxy-io/dls/theme/tokens/t";

import { TRANSFORM_ACTION } from "@/pages/pipelines/components/transform/constants";
import {
  formatTransformStep,
  TransformSummaryPartKind,
} from "@/pages/pipelines/components/transform/grammar/format";
import { usePipelineTransformFieldsEnvironment } from "@/pages/pipelines/components/transform/PipelineTransformFieldsProvider";
import type { TransformStep } from "@/pages/pipelines/components/transform/types";

const Sentence = styled.div`
  display: flex;
  align-items: center;
  min-height: ${TRANSFORM_ACTION}px;
  min-width: 0;

  font-family: ${t.font.sans.family};
  font-size: ${t.font.sans.size.body_md};
  line-height: ${TRANSFORM_ACTION}px;
  letter-spacing: ${t.font.sans.spacing.body_md};
  color: ${t.color.text.secondary};
  overflow-wrap: anywhere;
`;

const ChipSlot = styled.span<{ $isRemoved: boolean }>`
  display: inline-flex;
  align-items: center;
  height: ${TRANSFORM_ACTION}px;
  padding: 0 2px;
  vertical-align: top;

  & span {
    font-family: ${t.font.mono.family};
    letter-spacing: ${t.font.mono.spacing.caption};
    text-decoration: ${({ $isRemoved }) => ($isRemoved ? "line-through" : "none")};
  }
`;

type TransformSummaryChipKind =
  | TransformSummaryPartKind.COLUMN
  | TransformSummaryPartKind.OUTPUT
  | TransformSummaryPartKind.REMOVED;

const SUMMARY_PART_KIND_TO_CHIP_MAP: Record<
  TransformSummaryChipKind,
  (label: string) => ReactElement
> = {
  [TransformSummaryPartKind.COLUMN]: (label) => (
    <Chip label={label} variant={ChipVariant.TERTIARY} size={ChipSize.SMALL} hasBorder />
  ),
  [TransformSummaryPartKind.OUTPUT]: (label) => (
    <Chip label={label} color="blue" size={ChipSize.SMALL} />
  ),
  [TransformSummaryPartKind.REMOVED]: (label) => (
    <Chip label={label} color="blue" size={ChipSize.SMALL} />
  ),
};

interface PipelineTransformFieldsStepSummaryProps {
  step: TransformStep;
}

const PipelineTransformFieldsStepSummary = ({ step }: PipelineTransformFieldsStepSummaryProps) => {
  const { functionsByName } = usePipelineTransformFieldsEnvironment();
  return (
    <Sentence>
      <Span>
        {formatTransformStep(step, functionsByName).map((part, index) =>
          match(part.kind)
            .with(
              TransformSummaryPartKind.COLUMN,
              TransformSummaryPartKind.OUTPUT,
              TransformSummaryPartKind.REMOVED,
              (kind) => (
                // biome-ignore lint/suspicious/noArrayIndexKey: parts are positional
                <ChipSlot key={index} $isRemoved={kind === TransformSummaryPartKind.REMOVED}>
                  {SUMMARY_PART_KIND_TO_CHIP_MAP[kind](part.text)}
                </ChipSlot>
              ),
            )
            .with(TransformSummaryPartKind.VALUE, () => (
              // biome-ignore lint/suspicious/noArrayIndexKey: parts are positional
              <Span key={index} variant={SpanVariant.PRIMARY} family={FontFamily.MONO}>
                {part.text}
              </Span>
            ))
            .with(TransformSummaryPartKind.VERB, () => (
              // biome-ignore lint/suspicious/noArrayIndexKey: parts are positional
              <Span key={index} variant={SpanVariant.PRIMARY} weight={TextWeight.MEDIUM}>
                {part.text}
              </Span>
            ))
            .with(TransformSummaryPartKind.TEXT, () => (
              // biome-ignore lint/suspicious/noArrayIndexKey: parts are positional
              <Fragment key={index}>{part.text}</Fragment>
            ))
            .exhaustive(),
        )}
      </Span>
    </Sentence>
  );
};

export default PipelineTransformFieldsStepSummary;
