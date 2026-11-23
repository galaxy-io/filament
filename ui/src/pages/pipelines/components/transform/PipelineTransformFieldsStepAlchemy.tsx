import { type FC, useMemo, useState } from "react";

import { ArrowCircleUpIcon, CubeIcon, PlusIcon } from "@phosphor-icons/react";

import SelectInput, {
  SelectInputSize,
  SelectInputVariant,
  type SelectOption,
} from "@galaxy-io/dls/inputs/SelectInput";

import type { AlchemyFunction } from "@/gen/ingestion/v1/alchemy_pb";

import {
  TRANSFORM_ALCHEMY_NEW_FUNCTION_OPTION_ID,
  TRANSFORM_ALCHEMY_UPDATE_VERSION_OPTION_ID,
} from "@/pages/pipelines/components/transform/constants";
import { usePipelineTransformFieldsAlchemyReturn } from "@/pages/pipelines/components/transform/hooks/usePipelineTransformFieldsAlchemyReturn";
import PipelineTransformFieldsAlchemyRuntimeIcon from "@/pages/pipelines/components/transform/PipelineTransformFieldsAlchemyRuntimeIcon";
import { usePipelineTransformFieldsEditor } from "@/pages/pipelines/components/transform/PipelineTransformFieldsProvider";
import type { TransformAlchemyStep } from "@/pages/pipelines/components/transform/types";
import {
  getAlchemyArrowContract,
  getAlchemyCreateFunctionHref,
  getAlchemyFunctionDisabledReason,
  hasAlchemyRuntimeIcon,
} from "@/pages/pipelines/components/transform/utils";

import { useGalaxyProductUrl } from "@/module/GalaxyProductsProvider";
import { GalaxyProduct } from "@/module/types";

import {
  createGetAlchemyFunctionInput,
  createListAlchemyFunctionsInput,
  useGetAlchemyFunctionQuery,
  useListAlchemyFunctionsInfiniteQuery,
} from "@/api/queries/alchemy";

const createFunctionOption = (fn: AlchemyFunction): SelectOption => {
  const disabledReason = getAlchemyFunctionDisabledReason(fn);
  return {
    id: fn.id,
    label: fn.name || fn.id,
    description: [fn.runtime, fn.currentVersion > 0n ? `v${fn.currentVersion}` : ""]
      .filter(Boolean)
      .join(" · "),
    isDisabled: disabledReason !== undefined,
    disabledReason,
    ...(hasAlchemyRuntimeIcon(fn)
      ? { leading: <PipelineTransformFieldsAlchemyRuntimeIcon alchemyFunction={fn} /> }
      : { icon: CubeIcon }),
  };
};

interface PipelineTransformFieldsStepAlchemyProps {
  step: TransformAlchemyStep;
  onChange: (step: TransformAlchemyStep) => void;
}

const PipelineTransformFieldsStepAlchemy: FC<PipelineTransformFieldsStepAlchemyProps> = ({
  step,
  onChange,
}) => {
  const { isDisabled, arrowSchema } = usePipelineTransformFieldsEditor();
  const alchemyUrl = useGalaxyProductUrl(GalaxyProduct.ALCHEMY);
  const contract = useMemo(() => getAlchemyArrowContract(arrowSchema), [arrowSchema]);
  const [search, setSearch] = useState("");
  const [request, setRequest] = useState<string | null>(null);
  usePipelineTransformFieldsAlchemyReturn(request, (functionId, version) => {
    setRequest(null);
    onChange({ ...step, function: functionId, version });
  });
  const { data, isLoading, isError, hasNextPage, isFetchingNextPage, fetchNextPage } =
    useListAlchemyFunctionsInfiniteQuery({ input: createListAlchemyFunctionsInput(search) });
  const { data: pinnedData } = useGetAlchemyFunctionQuery({
    input: createGetAlchemyFunctionInput(step.function, step.version),
    options: { enabled: step.function !== "" && step.version > 0 },
  });
  const functions = data?.pages.flatMap((page) => page.functions) ?? [];
  const selected = pinnedData?.function;
  const latest = selected ? Number(selected.currentVersion) : 0;
  const hasUpdate = step.version > 0 && latest > step.version;
  const pinnedOption: SelectOption = {
    ...(selected ? createFunctionOption(selected) : { id: step.function, icon: CubeIcon }),
    label: `${selected?.name || step.function}@v${step.version}`,
  };

  const functionOptions = functions.map((fn) =>
    fn.id === step.function ? pinnedOption : createFunctionOption(fn),
  );
  if (step.function !== "" && !functions.some((fn) => fn.id === step.function)) {
    functionOptions.push(pinnedOption);
  }
  const pinned: SelectOption[] = [
    ...(hasUpdate
      ? [
          {
            id: TRANSFORM_ALCHEMY_UPDATE_VERSION_OPTION_ID,
            label: `Update to v${latest}`,
            icon: ArrowCircleUpIcon,
          },
        ]
      : []),
    ...(alchemyUrl
      ? [{ id: TRANSFORM_ALCHEMY_NEW_FUNCTION_OPTION_ID, label: "New function", icon: PlusIcon }]
      : []),
  ];

  return (
    <SelectInput
      ariaLabel="Alchemy function"
      options={[...pinned, ...functionOptions]}
      value={step.function || null}
      onChange={(id) => {
        if (id === TRANSFORM_ALCHEMY_NEW_FUNCTION_OPTION_ID) {
          if (!alchemyUrl) return;
          const next = crypto.randomUUID();
          setRequest(next);
          window.open(
            getAlchemyCreateFunctionHref(alchemyUrl, window.location.href, next, contract),
            "_blank",
            "noopener,noreferrer",
          );
          return;
        }
        if (id === TRANSFORM_ALCHEMY_UPDATE_VERSION_OPTION_ID) {
          onChange({ ...step, version: latest });
          return;
        }
        if (id === step.function) return;
        const fn = functions.find((candidate) => candidate.id === id);
        onChange({ ...step, function: fn?.id ?? "", version: fn ? Number(fn.currentVersion) : 0 });
      }}
      pinnedIds={pinned.map((option) => option.id)}
      isSearchable={!isDisabled}
      onSearch={setSearch}
      onEndReached={() => {
        if (hasNextPage) void fetchNextPage();
      }}
      isLoading={isLoading || isFetchingNextPage}
      placeholder={isError ? "Alchemy is unavailable" : "Choose a function..."}
      emptyMessage={search ? "No matching functions" : "No functions yet"}
      variant={SelectInputVariant.TERTIARY}
      size={SelectInputSize.MEDIUM}
      isError={isError}
      isDisabled={isDisabled}
      fillWidth
    />
  );
};

export default PipelineTransformFieldsStepAlchemy;
