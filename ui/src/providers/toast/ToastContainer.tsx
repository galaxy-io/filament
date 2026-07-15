import type { ReactNode } from "react";

import { styled } from "@linaria/react";

import { TOAST_Z_INDEX } from "@galaxy-io/dls/overlay/constants";
import { getCssSizeValue } from "@galaxy-io/dls/utils/linaria";

interface ToastContainerProps {
  children: ReactNode;
  top?: number | string;
  right?: number | string;
  bottom?: number | string;
  left?: number | string;
}

interface StyledToastContainerProps {
  $top?: number | string;
  $right?: number | string;
  $bottom?: number | string;
  $left?: number | string;
}

const DEFAULT_OFFSET = 24;

const StyledToastContainer = styled.div<StyledToastContainerProps>`
  position: fixed;
  z-index: ${TOAST_Z_INDEX};

  top: ${({ $top }) => ($top !== undefined ? getCssSizeValue($top) : "auto")};
  right: ${({ $right, $left }) =>
    $right !== undefined
      ? getCssSizeValue($right)
      : $left !== undefined
        ? "auto"
        : getCssSizeValue(DEFAULT_OFFSET)};
  bottom: ${({ $bottom, $top }) =>
    $bottom !== undefined
      ? getCssSizeValue($bottom)
      : $top !== undefined
        ? "auto"
        : getCssSizeValue(DEFAULT_OFFSET)};
  left: ${({ $left }) => ($left !== undefined ? getCssSizeValue($left) : "auto")};

  display: flex;
  flex-direction: column-reverse;
  gap: 8px;

  pointer-events: none;

  > * {
    pointer-events: auto;
  }
`;

const ToastContainer = ({
  children,
  top,
  right,
  bottom,
  left,
}: ToastContainerProps) => {
  return (
    <StyledToastContainer $top={top} $right={right} $bottom={bottom} $left={left}>
      {children}
    </StyledToastContainer>
  );
};

export default ToastContainer;
