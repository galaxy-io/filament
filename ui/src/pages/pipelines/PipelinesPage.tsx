import { useMemo } from "react";

import { PlusIcon } from "@phosphor-icons/react";
import { useNavigate, useSearch } from "@tanstack/react-router";

import Button, { ButtonSize, ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import FlexWrapper from "@galaxy-io/dls/containers/FlexWrapper";

import DocsButton from "@/components/DocsButton";

import MainLayoutListPage from "@/layouts/main/MainLayoutListPage";

import PipelinesPageEmptyGraphic from "@/pages/pipelines/components/PipelinesPageEmptyGraphic";
import PipelinesTable from "@/pages/pipelines/components/table/PipelinesTable";

import { Flow } from "@/routes/__root";

import { useSuspenseListPipelinesInfiniteQuery } from "@/api/queries/pipelines";

import { isSearchMatch } from "@/utils/search";

const PipelinesPage = () => {
  const navigate = useNavigate();
  const { q = "" } = useSearch({ from: "/_main/pipelines" });

  const { data, hasNextPage, isFetchingNextPage, fetchNextPage } =
    useSuspenseListPipelinesInfiniteQuery({
      input: { includeLastRun: true },
    });

  const pipelines = useMemo(() => data.pages.flatMap((page) => page.pipelines), [data.pages]);

  const visiblePipelines = useMemo(
    () => pipelines.filter((item) => isSearchMatch(q, item.name, item.id)),
    [pipelines, q],
  );

  const handleNewPipeline = () => {
    void navigate({
      to: ".",
      search: (prev) => ({ ...prev, connectionId: undefined, flow: Flow.CREATE_PIPELINE }),
    });
  };

  const renderContent = () => {
    if (!pipelines.length) {
      return (
        <PipelinesPageEmptyGraphic
          actions={
            <FlexWrapper gap={8}>
              <Button
                label="New pipeline"
                icon={PlusIcon}
                variant={ButtonVariant.PRIMARY}
                size={ButtonSize.LARGE}
                onClick={handleNewPipeline}
              />
              <DocsButton label="Read the docs" path="/pipelines" size={ButtonSize.LARGE} />
            </FlexWrapper>
          }
        />
      );
    }

    return (
      <PipelinesTable
        pipelines={visiblePipelines}
        hasNextPage={hasNextPage}
        isFetchingNextPage={isFetchingNextPage}
        fetchNextPage={fetchNextPage}
      />
    );
  };

  return (
    <MainLayoutListPage
      actions={[
        <Button
          key="new-pipeline"
          label="New pipeline"
          icon={PlusIcon}
          variant={ButtonVariant.PRIMARY}
          onClick={handleNewPipeline}
        />,
      ]}
      noPadding
    >
      {renderContent()}
    </MainLayoutListPage>
  );
};

export default PipelinesPage;
