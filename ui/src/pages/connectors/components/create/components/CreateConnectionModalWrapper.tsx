import type { PropsWithChildren } from "react";

import { styled } from "@linaria/react";
import { match } from "ts-pattern";

import { withTheme } from "@galaxy-io/dls/theme/GalaxyTheme";
import type { PropsWithTheme } from "@galaxy-io/dls/theme/types";

import {
  CREATE_CONNECTION_MODAL_MAX_HEIGHT,
  CREATE_CONNECTION_MODAL_SELECTOR_MIN_HEIGHT,
  CREATE_CONNECTION_MODAL_WIDTH,
} from "@/pages/connectors/constants";
import { CreateConnectionModalStep } from "@/pages/connectors/components/create/types";

interface CreateConnectionModalWrapperProps {
  step: CreateConnectionModalStep;
}

const Wrapper = withTheme(styled.div<PropsWithTheme & { $step: CreateConnectionModalStep }>`
  display: flex;
  flex-direction: column;

  min-height: ${({ $step }) =>
    $step === CreateConnectionModalStep.SELECT ? CREATE_CONNECTION_MODAL_SELECTOR_MIN_HEIGHT : 0}px;
  max-height: ${CREATE_CONNECTION_MODAL_MAX_HEIGHT}px;
  width: ${({ $step }) => {
    return match($step)
      .with(CreateConnectionModalStep.SELECT, () => 900)
      .with(CreateConnectionModalStep.CONFIGURE, () => CREATE_CONNECTION_MODAL_WIDTH)
      .exhaustive();
  }}px;

  background-color: ${({ theme }) => theme.color.background.primary};
  border: 0.5px solid ${({ theme }) => theme.color.border.primary};
  border-radius: 8px;
  overflow: hidden;
`);

const CreateConnectionModalWrapper = ({
  children,
  step,
}: PropsWithChildren<CreateConnectionModalWrapperProps>) => {
  return <Wrapper $step={step}>{children}</Wrapper>;
};

export default CreateConnectionModalWrapper;
