import type { PropsWithChildren } from "react";

import { styled } from "@linaria/react";

import { withTheme } from "@galaxy-io/dls/theme/GalaxyTheme";
import type { PropsWithTheme } from "@galaxy-io/dls/theme/types";

import { useAuthLayoutAsideField } from "@/layouts/auth/hooks/useAuthLayoutAsideField";

const FieldWrapper = withTheme(styled.div<PropsWithTheme>`
  position: relative;
  flex: 1;
  min-width: 0;

  display: flex;
  align-items: center;
  justify-content: center;

  overflow: hidden;

  background-color: ${({ theme }) => theme.color.background.primaryAlt};
`);

const FieldCanvas = styled.canvas`
  position: absolute;
  inset: 0;

  display: block;
  width: 100%;
  height: 100%;
`;

const FieldContent = styled.div`
  position: relative;
`;

const AuthLayoutAsideField = ({ children }: PropsWithChildren) => {
  const { wrapperRef, canvasRef } = useAuthLayoutAsideField();

  return (
    <FieldWrapper ref={wrapperRef}>
      <FieldCanvas ref={canvasRef} />
      <FieldContent>{children}</FieldContent>
    </FieldWrapper>
  );
};

export default AuthLayoutAsideField;
