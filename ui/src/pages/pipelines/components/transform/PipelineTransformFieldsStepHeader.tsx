import type { FC, MouseEvent, PropsWithChildren } from "react";

import { styled } from "@linaria/react";
import { CaretDownIcon, DotsSixVerticalIcon } from "@phosphor-icons/react";

import Button, { ButtonSize, ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import Icon, { IconVariant } from "@galaxy-io/dls/icons/Icon";
import { FOCUS_RING, INTERACTIVE_RESET } from "@galaxy-io/dls/styles/mixins";
import { t } from "@galaxy-io/dls/theme/tokens/t";
import Rotate from "@galaxy-io/dls/transform/Rotate";

import {
  TRANSFORM_ACTION,
  TRANSFORM_GAP,
  TRANSFORM_HANDLE,
  TRANSFORM_HEADER_PADDING_X,
  TRANSFORM_HEADER_PADDING_Y,
} from "@/pages/pipelines/components/transform/constants";
import PipelineTransformFieldsRow, {
  PipelineTransformFieldsRowVariant,
} from "@/pages/pipelines/components/transform/PipelineTransformFieldsRow";
import type { PipelineTransformFieldsStepHandle } from "@/pages/pipelines/components/transform/types";

const CARET_ROTATION_DEG = -180;
const Header = styled.div<{ $isClickable: boolean; $isOpen: boolean }>`
  width: 100%;
  min-width: 0;
  padding: ${TRANSFORM_HEADER_PADDING_Y}px ${TRANSFORM_HEADER_PADDING_X}px
    ${({ $isOpen }) => ($isOpen ? TRANSFORM_GAP : TRANSFORM_HEADER_PADDING_Y)}px;

  cursor: ${({ $isClickable }) => ($isClickable ? "pointer" : "default")};
`;

const Handle = styled.button`
  ${INTERACTIVE_RESET}
  ${FOCUS_RING}
  display: flex;
  align-items: center;
  justify-content: center;
  width: ${TRANSFORM_HANDLE}px;
  height: ${TRANSFORM_ACTION}px;
  border-radius: ${t.radius.md};

  cursor: grab;
  transition: background-color ${t.duration.fast};

  &:hover {
    background-color: ${t.color.background.hovered};
  }

  &:active {
    cursor: grabbing;
  }

  &:disabled {
    background-color: transparent;
    cursor: default;
  }
`;

const stopPropagation = (event: MouseEvent) => event.stopPropagation();

interface PipelineTransformFieldsStepHeaderProps {
  isOpen: boolean;
  onToggle?: () => void;
  handle?: PipelineTransformFieldsStepHandle;
}

const PipelineTransformFieldsStepHeader: FC<
  PropsWithChildren<PipelineTransformFieldsStepHeaderProps>
> = ({ isOpen, onToggle, handle, children }) => (
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
          <Rotate isRotated={isOpen} deg={CARET_ROTATION_DEG}>
            <Button
              icon={CaretDownIcon}
              variant={ButtonVariant.TERTIARY}
              size={ButtonSize.SMALL}
              onClick={(event) => {
                event.stopPropagation();
                onToggle();
              }}
              ariaLabel={isOpen ? "Collapse" : "Expand"}
              tooltip={isOpen ? "Collapse" : "Expand"}
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
