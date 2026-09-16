import type { ComponentProps, PropsWithChildren, ReactNode } from "react";

import Accordion, { AccordionVariant } from "@galaxy-io/dls/accordion/Accordion";

import EmptyLayout, { EmptyLayoutSize } from "@/layouts/EmptyLayout";

interface PipelineCanvasPanelSectionProps {
  header: string;
  isEmpty: boolean;
  emptyHeader: string;
  emptyMessage: string;
  padding?: ComponentProps<typeof Accordion>["padding"];
  isOpenInitial?: boolean;
  /** Trailing content in the header, such as an action for the section. */
  metric?: ReactNode;
  /** Controlled open state; pass with onToggle. */
  isOpen?: boolean;
  onToggle?: () => void;
}

const PipelineCanvasPanelSection = ({
  header,
  isEmpty,
  emptyHeader,
  emptyMessage,
  padding = 0,
  isOpenInitial = true,
  metric,
  isOpen,
  onToggle,
  children,
}: PropsWithChildren<PipelineCanvasPanelSectionProps>) => (
  <Accordion
    header={header}
    variant={AccordionVariant.TERTIARY}
    padding={isEmpty ? "24px" : padding}
    isOpenInitial={isOpenInitial}
    isOpen={isOpen}
    onToggle={onToggle}
    metric={metric}
  >
    {isEmpty ? (
      <EmptyLayout size={EmptyLayoutSize.SMALL} header={emptyHeader} message={emptyMessage} />
    ) : (
      children
    )}
  </Accordion>
);

export default PipelineCanvasPanelSection;
