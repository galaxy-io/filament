import { styled } from "@linaria/react";
import { match } from "ts-pattern";

import Chip, { ChipSize, ChipVariant } from "@galaxy-io/dls/chips/Chip";
import { withTheme } from "@galaxy-io/dls/theme/GalaxyTheme";
import type { PropsWithTheme } from "@galaxy-io/dls/theme/types";

import { TRANSFORM_ACTION } from "@/pages/pipelines/components/transform/constants";
import {
  formatTransformStep,
  TransformSummaryPartKind,
} from "@/pages/pipelines/components/transform/grammar/format";
import { usePipelineTransformFieldsEnvironment } from "@/pages/pipelines/components/transform/PipelineTransformFieldsProvider";
import type { TransformStep } from "@/pages/pipelines/components/transform/types";

const Sentence = withTheme(styled.div<PropsWithTheme>`
  display: flex;
  align-items: center;
  min-height: ${TRANSFORM_ACTION}px;
  min-width: 0;

  font-family: ${({ theme }) => theme.font.sans.family};
  font-size: ${({ theme }) => theme.font.sans.size.body_md};
  line-height: ${TRANSFORM_ACTION}px;
  letter-spacing: ${({ theme }) => theme.font.sans.spacing.body_md};
  color: ${({ theme }) => theme.color.text.secondary};
  overflow-wrap: anywhere;
`);

const Verb = withTheme(styled.span<PropsWithTheme>`
  color: ${({ theme }) => theme.color.text.primary};
`);

const ChipSlot = withTheme(styled.span<PropsWithTheme>`
  display: inline-flex;
  align-items: center;
  height: ${TRANSFORM_ACTION}px;
  padding: 0 2px;
  vertical-align: top;

  & p {
    font-family: ${({ theme }) => theme.font.mono.family};
    letter-spacing: ${({ theme }) => theme.font.mono.spacing.caption};
  }
`);

const SUMMARY_PART_KIND_TO_CHIP_VARIANT_MAP: Record<
  TransformSummaryPartKind.COLUMN | TransformSummaryPartKind.OUTPUT,
  ChipVariant
> = {
  [TransformSummaryPartKind.COLUMN]: ChipVariant.BLUE,
  [TransformSummaryPartKind.OUTPUT]: ChipVariant.PRIMARY,
};

interface PipelineTransformFieldsStepSummaryProps {
  step: TransformStep;
}

const PipelineTransformFieldsStepSummary = ({ step }: PipelineTransformFieldsStepSummaryProps) => {
  const { functionsByName } = usePipelineTransformFieldsEnvironment();
  return (
    <Sentence>
      <span>
        {formatTransformStep(step, functionsByName).map((part, index) =>
          match(part.kind)
            .with(TransformSummaryPartKind.COLUMN, TransformSummaryPartKind.OUTPUT, (kind) => (
              // biome-ignore lint/suspicious/noArrayIndexKey: parts are positional
              <ChipSlot key={index}>
                <Chip
                  label={part.text}
                  variant={SUMMARY_PART_KIND_TO_CHIP_VARIANT_MAP[kind]}
                  size={ChipSize.SMALL}
                />
              </ChipSlot>
            ))
            .with(TransformSummaryPartKind.VERB, () => (
              // biome-ignore lint/suspicious/noArrayIndexKey: parts are positional
              <Verb key={index}>{part.text}</Verb>
            ))
            .with(TransformSummaryPartKind.TEXT, () => (
              // biome-ignore lint/suspicious/noArrayIndexKey: parts are positional
              <span key={index}>{part.text}</span>
            ))
            .exhaustive(),
        )}
      </span>
    </Sentence>
  );
};

export default PipelineTransformFieldsStepSummary;
