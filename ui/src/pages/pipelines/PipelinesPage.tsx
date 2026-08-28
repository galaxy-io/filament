import { useMemo } from "react";

import { PlusIcon } from "@phosphor-icons/react";
import { useNavigate, useSearch } from "@tanstack/react-router";

import Button, { ButtonSize, ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import FlexWrapper from "@galaxy-io/dls/containers/FlexWrapper";

import DocsButton from "@/components/DocsButton";

import MainLayoutListPage from "@/layouts/main/MainLayoutListPage";

import PipelinesPageEmptyGraphic from "@/pages/pipelines/components/PipelinesPageEmptyGraphic";
import PipelinesTable from "@/pages/pipelines/components/table/PipelinesTable";
import {
  createPipelinesTableSorting,
  createPipelinesTableSortSearch,
  type PipelinesTableSortingChange,
} from "@/pages/pipelines/components/table/utils";

import { Flow } from "@/routes/__root";

import {
  createListPipelinesInput,
  useSuspenseListPipelinesInfiniteQuery,
} from "@/api/queries/pipelines";

const PipelinesPage = () => {
  const navigate = useNavigate();
  const search = useSearch({ from: "/_main/pipelines" });

  const { data, hasNextPage, isFetchingNextPage, fetchNextPage } =
    useSuspenseListPipelinesInfiniteQuery({
      input: createListPipelinesInput(search),
    });

  const pipelines = useMemo(() => data.pages.flatMap((page) => page.pipelines), [data.pages]);

  const sorting = useMemo(() => createPipelinesTableSorting(search), [search]);

  const handleSortingChange: PipelinesTableSortingChange = (updater) => {
    const next = typeof updater === "function" ? updater(sorting) : updater;
    void navigate({
      to: ".",
      replace: true,
      search: (prev) => ({ ...prev, ...createPipelinesTableSortSearch(next) }),
    });
  };

  const handleNewPipeline = () => {
    void navigate({
      to: ".",
      search: (prev) => ({ ...prev, connectionId: undefined, flow: Flow.CREATE_PIPELINE }),
    });
  };

  const renderContent = () => {
    if (!pipelines.length && !search.q) {
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
        pipelines={pipelines}
        sorting={sorting}
        onSortingChange={handleSortingChange}
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
