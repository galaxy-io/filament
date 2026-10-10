import type { FC } from "react";

import Tabs, { type TabLinkItem } from "@galaxy-io/dls/navigation/Tabs";

import RouterLink from "@/components/RouterLink";

import {
  PIPELINE_PAGE_TAB_TO_ICON_MAP,
  PIPELINE_PAGE_TAB_TO_LABEL_MAP,
  PIPELINE_PAGE_TAB_TO_PATH_MAP,
  PIPELINE_PAGE_TABS,
  PIPELINE_PAGE_TABS_INSET,
} from "@/pages/pipelines/components/header/constants";
import { PipelinePageTab } from "@/pages/pipelines/components/header/types";

import { useFilamentMatchRoute, usePipelineParams } from "@/module/hooks";
import { createFilamentHref } from "@/module/paths";

const PipelinePageTabs: FC = () => {
  const { id } = usePipelineParams();
  const matchRoute = useFilamentMatchRoute();

  const items: TabLinkItem<PipelinePageTab>[] = PIPELINE_PAGE_TABS.map((tab) => ({
    id: tab,
    label: PIPELINE_PAGE_TAB_TO_LABEL_MAP[tab],
    icon: PIPELINE_PAGE_TAB_TO_ICON_MAP[tab],
    href: createFilamentHref(PIPELINE_PAGE_TAB_TO_PATH_MAP[tab], { id }),
  }));

  const activeTab =
    PIPELINE_PAGE_TABS.find((tab) =>
      matchRoute(PIPELINE_PAGE_TAB_TO_PATH_MAP[tab], { params: { id } }),
    ) ?? PipelinePageTab.CANVAS;

  return (
    <Tabs
      ariaLabel="Pipeline"
      as={RouterLink}
      items={items}
      value={activeTab}
      inset={PIPELINE_PAGE_TABS_INSET}
    />
  );
};

export default PipelinePageTabs;
