import type { PropsWithChildren } from "react";

import { styled } from "@linaria/react";

import { t } from "@galaxy-io/dls/theme/tokens/t";

import { useAuthLayoutAsideField } from "@/layouts/auth/hooks/useAuthLayoutAsideField";

// @dls-migrate tokens.background.alt: Inverse is a scope, not a token: render the opposite-theme surface as `<GalaxyProvider isScoped theme={…}>` around a `Box variant`, and read the normal roles inside it.
const FieldWrapper = styled.div`
  position: relative;
  flex: 1;
  min-width: 0;

  display: flex;
  align-items: center;
  justify-content: center;

  overflow: hidden;

  background-color: ${t.color.background.primaryAlt};
`;

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
