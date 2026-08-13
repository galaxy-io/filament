import type { ComponentProps, PropsWithChildren } from "react";

import Accordion from "@galaxy-io/dls/accordion/Accordion";

import EmptyLayout, { EmptyLayoutSize } from "@/layouts/EmptyLayout";

interface PipelineCanvasPanelSectionProps {
  header: string;
  isEmpty: boolean;
  emptyHeader: string;
  emptyMessage: string;
  padding?: ComponentProps<typeof Accordion>["padding"];
  isOpenInitial?: boolean;
}

const PipelineCanvasPanelSection = ({
  header,
  isEmpty,
  emptyHeader,
  emptyMessage,
  padding = 0,
  isOpenInitial = true,
  children,
}: PropsWithChildren<PipelineCanvasPanelSectionProps>) => (
  <Accordion header={header} padding={isEmpty ? "24px" : padding} isOpenInitial={isOpenInitial}>
    {isEmpty ? (
      <EmptyLayout size={EmptyLayoutSize.SMALL} header={emptyHeader} message={emptyMessage} />
    ) : (
      children
    )}
  </Accordion>
);

export default PipelineCanvasPanelSection;
