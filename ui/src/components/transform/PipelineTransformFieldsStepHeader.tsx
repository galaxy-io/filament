import type { MouseEvent, PropsWithChildren } from "react";

import { styled } from "@linaria/react";
import { CaretDownIcon, DotsSixVerticalIcon } from "@phosphor-icons/react";

import Button, { ButtonSize, ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import Icon, { IconVariant } from "@galaxy-io/dls/icons/Icon";
import { withTheme } from "@galaxy-io/dls/theme/GalaxyTheme";
import type { PropsWithTheme } from "@galaxy-io/dls/theme/types";
import Rotate from "@galaxy-io/dls/transform/Rotate";

import {
  TRANSFORM_ACTION,
  TRANSFORM_GAP,
  TRANSFORM_HANDLE,
  TRANSFORM_HEADER_PADDING_X,
  TRANSFORM_HEADER_PADDING_Y,
} from "@/components/transform/constants";
import PipelineTransformFieldsRow, {
  PipelineTransformFieldsRowVariant,
} from "@/components/transform/PipelineTransformFieldsRow";
import type { PipelineTransformFieldsStepHandle } from "@/components/transform/types";

const CARET_ROTATION_DEG = -180;
const CARET_ROTATION_SPEED = 0.2;

export const PipelineTransformFieldsStepSurface = withTheme(styled.div<
  PropsWithTheme<{ $isHoverable: boolean }>
>`
  width: 100%;
  min-width: 0;

  transition: background-color 100ms ease;

  &:hover {
    background-color: ${({ theme, $isHoverable }) =>
      $isHoverable ? theme.color.background.secondary : "transparent"};
  }
`);

const Header = styled.div<{ $isClickable: boolean; $isOpen: boolean }>`
  width: 100%;
  min-width: 0;
  padding: ${TRANSFORM_HEADER_PADDING_Y}px ${TRANSFORM_HEADER_PADDING_X}px
    ${({ $isOpen }) => ($isOpen ? TRANSFORM_GAP : TRANSFORM_HEADER_PADDING_Y)}px;

  cursor: ${({ $isClickable }) => ($isClickable ? "pointer" : "default")};
`;

const Handle = withTheme(styled.button<PropsWithTheme>`
  display: flex;
  align-items: center;
  justify-content: center;
  width: ${TRANSFORM_HANDLE}px;
  height: ${TRANSFORM_ACTION}px;
  padding: 0;
  border: 0;
  border-radius: 4px;

  background-color: transparent;
  color: inherit;
  cursor: grab;
  transition: background-color 100ms ease;

  &:hover {
    background-color: ${({ theme }) => theme.color.background.tertiary};
  }

  &:active {
    cursor: grabbing;
  }

  &:focus-visible {
    outline: 2px solid ${({ theme }) => theme.color.border.selected};
    outline-offset: -2px;
  }

  &:disabled {
    background-color: transparent;
    cursor: default;
  }
`);

const stopPropagation = (event: MouseEvent) => event.stopPropagation();

interface PipelineTransformFieldsStepHeaderProps {
  isOpen: boolean;
  onToggle?: () => void;
  handle?: PipelineTransformFieldsStepHandle;
}

const PipelineTransformFieldsStepHeader = ({
  isOpen,
  onToggle,
  handle,
  children,
}: PropsWithChildren<PipelineTransformFieldsStepHeaderProps>) => (
  <Header
    $isClickable={!isOpen && onToggle !== undefined}
    $isOpen={isOpen}
    onClick={isOpen ? undefined : onToggle}
  >
    <PipelineTransformFieldsRow
      variant={PipelineTransformFieldsRowVariant.HEADER}
      gutter={
        <Handle
          ref={handle?.ref}
          type="button"
          aria-label="Drag to reorder"
          disabled={handle === undefined}
          onClick={stopPropagation}
          {...handle?.attributes}
          {...handle?.listeners}
        >
          <Icon component={DotsSixVerticalIcon} variant={IconVariant.SECONDARY} />
        </Handle>
      }
      action={
        onToggle ? (
          <Rotate isRotated={isOpen} deg={CARET_ROTATION_DEG} speed={CARET_ROTATION_SPEED}>
            <Button
              icon={CaretDownIcon}
              variant={ButtonVariant.TERTIARY}
              size={ButtonSize.SMALL}
              onClick={isOpen ? onToggle : stopPropagation}
              ariaLabel={isOpen ? "Collapse" : "Expand"}
            />
          </Rotate>
        ) : undefined
      }
    >
      {children}
    </PipelineTransformFieldsRow>
  </Header>
);

export default PipelineTransformFieldsStepHeader;
