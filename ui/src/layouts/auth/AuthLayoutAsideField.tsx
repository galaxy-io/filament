import type { PropsWithChildren } from "react";

import { styled } from "@linaria/react";

import { GalaxyTheme } from "@galaxy-io/dls/theme/enums";
import GalaxyProvider from "@galaxy-io/dls/theme/GalaxyProvider";
import { t } from "@galaxy-io/dls/theme/tokens/t";
import { useGalaxyTheme } from "@galaxy-io/dls/theme/useGalaxyTheme";

import { useAuthLayoutAsideField } from "@/layouts/auth/hooks/useAuthLayoutAsideField";

const FieldWrapper = styled.div`
  position: relative;
  flex: 1;
  min-width: 0;
  overflow: hidden;

  & > [data-gx-theme] {
    position: absolute;
    inset: 0;

    display: flex;
    align-items: center;
    justify-content: center;

    background-color: ${t.color.background.primary};
  }
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
  const { activeTheme } = useGalaxyTheme();
  const inverseTheme = activeTheme === GalaxyTheme.LIGHT ? GalaxyTheme.DARK : GalaxyTheme.LIGHT;

  return (
    <FieldWrapper ref={wrapperRef}>
      <GalaxyProvider key={inverseTheme} theme={inverseTheme} isScoped>
        <FieldCanvas ref={canvasRef} />
        <FieldContent>{children}</FieldContent>
      </GalaxyProvider>
    </FieldWrapper>
  );
};

export default AuthLayoutAsideField;
