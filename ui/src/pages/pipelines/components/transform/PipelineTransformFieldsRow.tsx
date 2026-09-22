import type { PropsWithChildren, ReactNode } from "react";

import { styled } from "@linaria/react";

import Text, { TextSize, TextVariant } from "@galaxy-io/dls/text/Text";

import {
  TRANSFORM_ACTION,
  TRANSFORM_ADD_ROW,
  TRANSFORM_BOX_PAD,
  TRANSFORM_COND_GUTTER,
  TRANSFORM_GAP,
  TRANSFORM_GUTTER,
  TRANSFORM_HANDLE,
} from "@/pages/pipelines/components/transform/constants";

export enum PipelineTransformFieldsRowVariant {
  STEP = "STEP",
  COND = "COND",
  HEADER = "HEADER",
  ARG = "ARG",
}

const ROW_VARIANT_TO_GUTTER_WIDTH_MAP: Record<PipelineTransformFieldsRowVariant, number> = {
  [PipelineTransformFieldsRowVariant.STEP]: TRANSFORM_GUTTER,
  [PipelineTransformFieldsRowVariant.COND]: TRANSFORM_COND_GUTTER,
  [PipelineTransformFieldsRowVariant.HEADER]: TRANSFORM_HANDLE,
  [PipelineTransformFieldsRowVariant.ARG]: 0,
};

const Grid = styled.div<{ $columns: string }>`
  display: grid;
  grid-template-columns: ${({ $columns }) => $columns};
  align-items: start;
  column-gap: ${TRANSFORM_GAP}px;

  width: 100%;
  min-width: 0;

  & > * {
    min-width: 0;
  }
`;

const Gutter = styled.div<{ $inset: number; $offset: number; $minHeight: number }>`
  display: flex;
  align-items: center;
  min-height: ${({ $minHeight }) => $minHeight}px;
  padding-top: ${({ $offset }) => $offset}px;
  padding-left: ${({ $inset }) => $inset}px;
  overflow: hidden;
`;

const Control = styled.div<{ $span: number; $minHeight: number }>`
  display: ${({ $minHeight }) => ($minHeight > 0 ? "flex" : "block")};
  align-items: center;
  grid-column: span ${({ $span }) => $span};
  min-height: ${({ $minHeight }) => $minHeight}px;
`;

const Action = styled.div<{ $offset: number }>`
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: ${TRANSFORM_ACTION}px;
  padding-top: ${({ $offset }) => $offset}px;
`;

interface PipelineTransformFieldsRowProps {
  variant: PipelineTransformFieldsRowVariant;
  gutter?: ReactNode;
  action?: ReactNode;
  isBoxed?: boolean;
  isAddRow?: boolean;
}

const PipelineTransformFieldsRow = ({
  variant,
  gutter,
  action,
  isBoxed = false,
  isAddRow = false,
  children,
}: PropsWithChildren<PipelineTransformFieldsRowProps>) => {
  const isArg = variant === PipelineTransformFieldsRowVariant.ARG;
  const hasGutter = isArg || gutter !== undefined;
  const hasAction = action !== undefined;
  const columns = isArg
    ? "subgrid"
    : [
        hasGutter ? `${ROW_VARIANT_TO_GUTTER_WIDTH_MAP[variant]}px` : "",
        "minmax(0, 1fr)",
        hasAction ? `${TRANSFORM_ACTION}px` : "",
      ]
        .filter(Boolean)
        .join(" ");
  const offset = isBoxed ? TRANSFORM_BOX_PAD : 0;
  const minHeight = isAddRow ? TRANSFORM_ADD_ROW : TRANSFORM_ACTION;

  return (
    <Grid $columns={columns}>
      {hasGutter && (
        <Gutter
          $inset={variant === PipelineTransformFieldsRowVariant.HEADER ? 0 : TRANSFORM_GAP}
          $offset={offset}
          $minHeight={minHeight}
        >
          {typeof gutter === "string" ? (
            <Text size={TextSize.BODY_SM} variant={TextVariant.TERTIARY} isEllipsis={isArg}>
              {gutter}
            </Text>
          ) : (
            gutter
          )}
        </Gutter>
      )}
      <Control $span={isArg && !hasAction ? 2 : 1} $minHeight={isAddRow ? minHeight : 0}>
        {children}
      </Control>
      {hasAction && <Action $offset={offset}>{action}</Action>}
    </Grid>
  );
};

export default PipelineTransformFieldsRow;
