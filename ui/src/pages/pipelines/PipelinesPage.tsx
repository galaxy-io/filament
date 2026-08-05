import { useMemo, useState } from "react";

import { BookOpenIcon, PlusIcon } from "@phosphor-icons/react";
import { useNavigate } from "@tanstack/react-router";

import Button, { ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import FlexWrapper from "@galaxy-io/dls/containers/FlexWrapper";

import MainLayoutListPage from "@/layouts/main/MainLayoutListPage";

import PipelinesPageEmptyGraphic from "@/pages/pipelines/components/PipelinesPageEmptyGraphic";
import PipelinesTable from "@/pages/pipelines/components/table/PipelinesTable";

import { Flow } from "@/routes/__root";

import { useSuspenseListPipelinesQuery } from "@/api/queries/pipelines";

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

  const handleSearchChange = (search: string) => {
    setState((prev) => ({ ...prev, search }));
  };

  const { data } = useSuspenseListPipelinesQuery();

  const visiblePipelines = useMemo(
    () => data.pipelines.filter((item) => isSearchMatch(state.search, item.name, item.id)),
    [data.pipelines, state.search],
  );

  const handleNewPipeline = () => {
    void navigate({
      to: ".",
      search: (prev) => ({ ...prev, flow: Flow.CREATE_PIPELINE }),
    });
  };

  const handleReadTheDocs = () => {
    window.open(DOCUMENTATION_URL, "_blank");
  };

  const renderContent = () => {
    if (!data.pipelines.length) {
      return (
        <PipelinesPageEmptyGraphic
          actions={
            <FlexWrapper gap={8}>
              <Button
                label="New pipeline"
                icon={PlusIcon}
                variant={ButtonVariant.PRIMARY}
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

    return <PipelinesTable pipelines={visiblePipelines} />;
  };

  return (
    <MainLayoutListPage
      search={state.search}
      onSearchChange={handleSearchChange}
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
