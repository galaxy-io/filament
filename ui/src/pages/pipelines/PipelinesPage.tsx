import { useMemo, useState } from "react";

import { create } from "@bufbuild/protobuf";
import { BookOpenIcon, MagnifyingGlassIcon, PlusIcon } from "@phosphor-icons/react";
import { useNavigate } from "@tanstack/react-router";

import Button, { ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import FlexWrapper from "@galaxy-io/dls/containers/FlexWrapper";
import Icon, { IconVariant } from "@galaxy-io/dls/icons/Icon";
import { ToastVariant } from "@galaxy-io/dls/toast/ToastProvider";
import { useToast } from "@galaxy-io/dls/toast/useToast";

import { CreatePipelineRequestSchema } from "@/gen/ingestion/v1/pipelines_pb";

import ConnectorEmptyDark from "@/assets/components/ConnectorsEmptyDark";

import EmptyLayout from "@/layouts/EmptyLayout";
import ListPageLayout from "@/layouts/ListPageLayout";

import PipelineCard from "@/pages/pipelines/components/card/PipelineCard";

import { useCreatePipelineMutation, useSuspenseListPipelinesQuery } from "@/api/queries/pipelines";

import { DOCUMENTATION_URL } from "@/constants";

import { isSearchMatch } from "@/utils/search";

interface PipelinesPageState {
  search: string;
}

const DEFAULT_STATE: PipelinesPageState = {
  search: "",
};

const PipelinesPage = () => {
  const navigate = useNavigate();

  const [state, setState] = useState<PipelinesPageState>(DEFAULT_STATE);
  const { showToast } = useToast();

  const handleSearchChange = (search: string) => {
    setState((prev) => ({ ...prev, search }));
  };

  const { data } = useSuspenseListPipelinesQuery();
  const { mutate: createPipeline, isPending: isCreatingPipeline } = useCreatePipelineMutation();

  const pipelines = data.pipelines;

  const visiblePipelines = useMemo(
    () => pipelines.filter((item) => isSearchMatch(state.search, item.name)),
    [pipelines, state.search],
  );

  const handleNewPipeline = () => {
    createPipeline(create(CreatePipelineRequestSchema, {}), {
      onSuccess: (response) => {
        if (response.pipeline?.id) {
          navigate({
            to: "/pipelines/$id",
            params: { id: response.pipeline.id },
          });
        }
      },
      onError: (error) => {
        showToast({
          variant: ToastVariant.ERROR,
          header: "Failed to create pipeline",
          subheader: error.message,
        });
      },
    });
  };

  const handleReadTheDocs = () => {
    window.open(DOCUMENTATION_URL, "_blank");
  };

  const renderContent = () => {
    if (!pipelines.length) {
      return (
        <EmptyLayout
          icon={<ConnectorEmptyDark height={200} />}
          header="No pipelines found"
          message="Create pipelines to move data between your connectors."
          actions={
            <FlexWrapper gap={8}>
              <Button
                label={isCreatingPipeline ? "Creating..." : "New pipeline"}
                icon={PlusIcon}
                variant={ButtonVariant.PRIMARY}
                isDisabled={isCreatingPipeline}
                onClick={handleNewPipeline}
              />
              <Button
                label="Documentation"
                icon={BookOpenIcon}
                variant={ButtonVariant.SECONDARY}
                onClick={handleReadTheDocs}
              />
            </FlexWrapper>
          }
        />
      );
    }

    if (visiblePipelines.length === 0) {
      return (
        <EmptyLayout
          icon={<Icon component={MagnifyingGlassIcon} variant={IconVariant.TERTIARY} />}
          message="No pipelines match your search"
        />
      );
    }

    return visiblePipelines.map((pipeline) => (
      <PipelineCard key={pipeline.id} pipeline={pipeline} />
    ));
  };

  return (
    <ListPageLayout
      search={state.search}
      onSearchChange={handleSearchChange}
      actions={[
        <Button
          key="new-pipeline"
          label={isCreatingPipeline ? "Creating..." : "New pipeline"}
          icon={PlusIcon}
          variant={ButtonVariant.PRIMARY}
          isDisabled={isCreatingPipeline}
          onClick={handleNewPipeline}
        />,
      ]}
      noPadding
    >
      {renderContent()}
    </ListPageLayout>
  );
};

export default PipelinesPage;
