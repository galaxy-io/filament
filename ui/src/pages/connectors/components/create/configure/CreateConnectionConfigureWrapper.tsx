import type { PropsWithChildren } from "react";

import { styled } from "@linaria/react";
import { match } from "ts-pattern";

import { withTheme } from "@galaxy-io/dls/theme/GalaxyTheme";
import type { PropsWithTheme } from "@galaxy-io/dls/theme/types";

import { CreateConnectionModalStep } from "@/pages/connectors/components/create/types";
import {
  CREATE_CONNECTION_MODAL_CONFIGURE_WIDTH,
  CREATE_CONNECTION_MODAL_MAX_HEIGHT,
  CREATE_CONNECTION_MODAL_MIN_HEIGHT,
  CREATE_CONNECTION_MODAL_SELECTOR_WIDTH,
} from "@/pages/connectors/constants";

interface CreateConnectionConfigureWrapperProps {
  step: CreateConnectionModalStep;
}

const Wrapper = withTheme(styled.div<PropsWithTheme & { $step: CreateConnectionModalStep }>`
  display: flex;
  flex-direction: column;

  min-height: ${CREATE_CONNECTION_MODAL_MIN_HEIGHT}px;
  max-height: ${CREATE_CONNECTION_MODAL_MAX_HEIGHT}px;
  width: ${({ $step }) => {
    return match($step)
      .with(CreateConnectionModalStep.SELECT, () => CREATE_CONNECTION_MODAL_SELECTOR_WIDTH)
      .with(CreateConnectionModalStep.CONFIGURE, () => CREATE_CONNECTION_MODAL_CONFIGURE_WIDTH)
      .exhaustive();
  }}px;

  background-color: ${({ theme }) => theme.color.background.primary};
  border: 0.5px solid ${({ theme }) => theme.color.border.primary};
  border-radius: 8px;
  overflow: hidden;
`);

const CreateConnectionConfigureWrapper = ({
  children,
  step,
}: PropsWithChildren<CreateConnectionConfigureWrapperProps>) => {
  return <Wrapper $step={step}>{children}</Wrapper>;
};

export default CreateConnectionConfigureWrapper;
