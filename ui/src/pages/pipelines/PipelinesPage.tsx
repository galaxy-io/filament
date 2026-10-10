import { type FC, useMemo } from "react";

import { PlusIcon } from "@phosphor-icons/react";

import Button, { ButtonSize, ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import Flex, { AlignItems } from "@galaxy-io/dls/layout/Flex";
import PageLayout from "@galaxy-io/dls/layout/PageLayout";
import type { TableSort } from "@galaxy-io/dls/table/types";

import DocsLink from "@/components/DocsLink";
import ListSearch from "@/components/ListSearch";
import { PIPELINE_CREATE_TITLE } from "@/components/pipelines/constants";

import PipelinesPageEmptyGraphic from "@/pages/pipelines/components/PipelinesPageEmptyGraphic";
import { PIPELINES_TABLE_SORT_BY_TO_COLUMN_ID_MAP } from "@/pages/pipelines/components/table/constants";
import PipelinesTable from "@/pages/pipelines/components/table/PipelinesTable";
import { PIPELINES_DOCS_PATH } from "@/pages/pipelines/constants";

import { useFilamentFlowOpen, useFilamentSearchUpdate, usePipelinesSearch } from "@/module/hooks";
import type { PipelinesSearch } from "@/module/schemas";
import { Flow } from "@/module/types";

import {
  createListPipelinesInput,
  useSuspenseListPipelinesInfiniteQuery,
} from "@/api/queries/pipelines";

import { createTableSorting, createTableSortSearch } from "@/utils/sort";

const PipelinesPage: FC = () => {
  const updateSearch = useFilamentSearchUpdate<PipelinesSearch>();
  const openFlow = useFilamentFlowOpen();
  const search = usePipelinesSearch();

  const { data, hasNextPage, isFetchingNextPage, fetchNextPage } =
    useSuspenseListPipelinesInfiniteQuery({
      input: createListPipelinesInput(search),
    });

  const pipelines = useMemo(() => data.pages.flatMap((page) => page.pipelines), [data.pages]);

  const sorting = useMemo(
    () => createTableSorting(search, PIPELINES_TABLE_SORT_BY_TO_COLUMN_ID_MAP),
    [search],
  );

  const handleSortingChange = (next: TableSort | null) => {
    void updateSearch(
      (prev) => ({
        ...prev,
        ...createTableSortSearch(next, PIPELINES_TABLE_SORT_BY_TO_COLUMN_ID_MAP),
      }),
      {
        replace: true,
      },
    );
  };

  const handleNewPipeline = () => {
    openFlow(Flow.CREATE_PIPELINE);
  };

  const renderContent = () => {
    if (!pipelines.length && !search.q) {
      return (
        <PipelinesPageEmptyGraphic
          actions={
            <Flex alignItems={AlignItems.CENTER} gap={16}>
              <Button
                label={PIPELINE_CREATE_TITLE}
                icon={PlusIcon}
                variant={ButtonVariant.PRIMARY}
                size={ButtonSize.LARGE}
                onClick={handleNewPipeline}
              />
              <DocsLink label="Learn about pipelines" path={PIPELINES_DOCS_PATH} />
            </Flex>
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
    <PageLayout
      header="Pipelines"
      actions={
        <Button
          label={PIPELINE_CREATE_TITLE}
          icon={PlusIcon}
          variant={ButtonVariant.PRIMARY}
          onClick={handleNewPipeline}
        />
      }
      toolbar={<ListSearch placeholder="Search pipelines" />}
    >
      {renderContent()}
    </PageLayout>
  );
};

export default PipelinesPage;
