import type { FC } from "react";

import { styled } from "@linaria/react";

import { GalaxyTheme } from "@galaxy-io/dls/theme/enums";
import { useGalaxyTheme } from "@galaxy-io/dls/theme/useGalaxyTheme";

import type { AlchemyFunction } from "@/gen/ingestion/v1/alchemy_pb";

import { TRANSFORM_ALCHEMY_RUNTIME_ICON_SIZE } from "@/pages/pipelines/components/transform/constants";

const Icon = styled.img`
  flex-shrink: 0;
  width: ${TRANSFORM_ALCHEMY_RUNTIME_ICON_SIZE}px;
  height: ${TRANSFORM_ALCHEMY_RUNTIME_ICON_SIZE}px;
`;

interface PipelineTransformFieldsAlchemyRuntimeIconProps {
  alchemyFunction: AlchemyFunction;
}

const PipelineTransformFieldsAlchemyRuntimeIcon: FC<
  PipelineTransformFieldsAlchemyRuntimeIconProps
> = ({ alchemyFunction }) => {
  const { activeTheme } = useGalaxyTheme();
  return (
    <Icon
      src={
        activeTheme === GalaxyTheme.DARK
          ? alchemyFunction.runtimeDarkIconUrl
          : alchemyFunction.runtimeLightIconUrl
      }
      alt={alchemyFunction.runtime}
    />
  );
};

export default PipelineTransformFieldsAlchemyRuntimeIcon;
