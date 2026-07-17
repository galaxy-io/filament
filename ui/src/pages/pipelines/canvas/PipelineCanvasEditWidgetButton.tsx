import { styled } from "@linaria/react";
import { match, P } from "ts-pattern";

import Icon, { IconVariant, IconWeight } from "@galaxy-io/dls/icons/Icon";
import { withTheme } from "@galaxy-io/dls/theme/GalaxyTheme";
import type { PropsWithTheme } from "@galaxy-io/dls/theme/types";

import { PIPELINE_CANVAS_EDIT_MODE_TO_ICON_MAP } from "@/pages/pipelines/canvas/constants";
import { PipelineCanvasEditMode } from "@/pages/pipelines/canvas/types";

const getIconProps = (mode: PipelineCanvasEditMode, isActive: boolean) =>
  match({ mode, isActive })
    .with({ mode: PipelineCanvasEditMode.ADD_NODE, isActive: true }, () => ({
      variant: IconVariant.PRIMARY_ALT,
    }))
    .with({ mode: PipelineCanvasEditMode.ADD_NODE, isActive: false }, () => ({
      variant: IconVariant.PRIMARY,
    }))
    .with(
      {
        mode: P.union(PipelineCanvasEditMode.ADD_EDGE, PipelineCanvasEditMode.GRAB),
        isActive: true,
      },
      () => ({
        variant: IconVariant.PRIMARY,
      }),
    )
    .with(
      {
        mode: P.union(PipelineCanvasEditMode.ADD_EDGE, PipelineCanvasEditMode.GRAB),
        isActive: false,
      },
      () => ({
        variant: IconVariant.TERTIARY,
      }),
    )
    .with({ mode: PipelineCanvasEditMode.ACTIVITY, isActive: true }, () => ({
      variant: undefined,
      weight: IconWeight.BOLD,
    }))
    .with({ mode: PipelineCanvasEditMode.ACTIVITY, isActive: false }, () => ({
      variant: IconVariant.TERTIARY,
      weight: IconWeight.REGULAR,
    }))
    .exhaustive();

const StyledButton = withTheme(
  styled.button<PropsWithTheme<{ $mode: PipelineCanvasEditMode; $isActive: boolean }>>`
    width: 32px;
    height: 32px;
    padding: 0;

    display: flex;
    align-items: center;
    justify-content: center;

    background-color: ${({ theme, $mode, $isActive }) =>
      match({ mode: $mode, isActive: $isActive })
        .with(
          { mode: PipelineCanvasEditMode.ADD_NODE, isActive: true },
          () => theme.color.background.primaryAlt,
        )
        .with(
          { mode: PipelineCanvasEditMode.ADD_NODE, isActive: false },
          () => theme.color.background.secondary,
        )
        .with(
          {
            mode: P.union(
              PipelineCanvasEditMode.ADD_EDGE,
              PipelineCanvasEditMode.GRAB,
              PipelineCanvasEditMode.ACTIVITY,
            ),
            isActive: true,
          },
          () => theme.color.background.secondary,
        )
        .with(
          {
            mode: P.union(
              PipelineCanvasEditMode.ADD_EDGE,
              PipelineCanvasEditMode.GRAB,
              PipelineCanvasEditMode.ACTIVITY,
            ),
            isActive: false,
          },
          () => theme.color.background.base,
        )
        .exhaustive()};
    border: none;
    border-radius: 50%;
    cursor: pointer;

    transition: background-color 100ms ease;

    &:hover {
      background-color: ${({ theme, $mode, $isActive }) =>
        match({ mode: $mode, isActive: $isActive })
          .with(
            { mode: PipelineCanvasEditMode.ADD_NODE },
            () => theme.color.background.secondaryAlt,
          )
          .with(
            {
              mode: P.union(
                PipelineCanvasEditMode.ADD_EDGE,
                PipelineCanvasEditMode.GRAB,
                PipelineCanvasEditMode.ACTIVITY,
              ),
              isActive: true,
            },
            () => theme.color.background.secondary,
          )
          .with(
            {
              mode: P.union(
                PipelineCanvasEditMode.ADD_EDGE,
                PipelineCanvasEditMode.GRAB,
                PipelineCanvasEditMode.ACTIVITY,
              ),
              isActive: false,
            },
            () => theme.color.background.tertiary,
          )
          .exhaustive()};
    }
  `,
);

interface PipelineCanvasEditWidgetButtonProps {
  mode: PipelineCanvasEditMode;
  isActive: boolean;
  onClick: () => void;
}

const PipelineCanvasEditWidgetButton = ({
  mode,
  isActive,
  onClick,
}: PipelineCanvasEditWidgetButtonProps) => (
  <StyledButton $mode={mode} $isActive={isActive} onClick={onClick}>
    <Icon
      component={PIPELINE_CANVAS_EDIT_MODE_TO_ICON_MAP[mode]}
      {...getIconProps(mode, isActive)}
    />
  </StyledButton>
);

export default PipelineCanvasEditWidgetButton;
