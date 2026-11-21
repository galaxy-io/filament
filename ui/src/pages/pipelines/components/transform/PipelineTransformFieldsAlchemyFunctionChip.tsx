import type { FC, MouseEvent } from "react";

import { CubeIcon } from "@phosphor-icons/react";

import Chip, { ChipSize, ChipVariant } from "@galaxy-io/dls/chips/Chip";

import type { AlchemyFunction } from "@/gen/ingestion/v1/alchemy_pb";

import PipelineTransformFieldsAlchemyRuntimeIcon from "@/pages/pipelines/components/transform/PipelineTransformFieldsAlchemyRuntimeIcon";
import {
  getAlchemyWorkbenchUrl,
  hasAlchemyRuntimeIcon,
} from "@/pages/pipelines/components/transform/utils";

import { useGalaxyProductUrl } from "@/module/GalaxyProductsProvider";
import { GalaxyProduct } from "@/module/types";

import { createGetAlchemyFunctionInput, useGetAlchemyFunctionQuery } from "@/api/queries/alchemy";

interface PipelineTransformFieldsAlchemyFunctionChipProps {
  functionId: AlchemyFunction["id"];
  version: number;
}

const PipelineTransformFieldsAlchemyFunctionChip: FC<
  PipelineTransformFieldsAlchemyFunctionChipProps
> = ({ functionId, version }) => {
  const { data } = useGetAlchemyFunctionQuery({ input: createGetAlchemyFunctionInput(functionId) });
  const alchemyUrl = useGalaxyProductUrl(GalaxyProduct.ALCHEMY);
  const fn = data?.function;
  const workbenchUrl = alchemyUrl
    ? getAlchemyWorkbenchUrl(alchemyUrl, functionId, fn?.runtimeHandlerPath)
    : undefined;
  return (
    <Chip
      label={`${fn?.name || functionId}@v${version}`}
      variant={ChipVariant.TERTIARY}
      size={ChipSize.SMALL}
      hasBorder
      {...(workbenchUrl && {
        tooltip: "Open in alchemy",
        onClick: (event: MouseEvent<HTMLButtonElement>) => {
          event.stopPropagation();
          window.open(workbenchUrl, "_blank", "noopener,noreferrer");
        },
      })}
      {...(fn && hasAlchemyRuntimeIcon(fn)
        ? { leading: <PipelineTransformFieldsAlchemyRuntimeIcon alchemyFunction={fn} /> }
        : { icon: CubeIcon })}
    />
  );
};

export default PipelineTransformFieldsAlchemyFunctionChip;
