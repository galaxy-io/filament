import type { ComponentProps, PropsWithChildren, ReactNode } from "react";

import Accordion, { AccordionVariant } from "@galaxy-io/dls/accordion/Accordion";
import FlexItem from "@galaxy-io/dls/containers/FlexItem";
import Wrapper from "@galaxy-io/dls/containers/Wrapper";

import EmptyLayout from "@/layouts/EmptyLayout";
import { LayoutSize } from "@/layouts/types";

interface PipelineCanvasPanelSectionProps {
  header: string;
  isEmpty: boolean;
  emptyHeader: string;
  emptyMessage: string;
  padding?: ComponentProps<typeof Accordion>["padding"];
  isOpenInitial?: boolean;
  metric?: ReactNode;
  headerAction?: ReactNode;
  headerActionWidth?: number;
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
  headerAction,
  headerActionWidth = 96,
  isOpen,
  onToggle,
  children,
}: PropsWithChildren<PipelineCanvasPanelSectionProps>) => {
  const accordion = (
    <Accordion
      header={header}
      variant={AccordionVariant.TERTIARY}
      padding={isEmpty ? "24px" : padding}
      isOpenInitial={isOpenInitial}
      isOpen={isOpen}
      onToggle={onToggle}
      metric={headerAction ? <FlexItem width={headerActionWidth} shrink={0} /> : metric}
    >
      {isEmpty ? (
        <EmptyLayout size={LayoutSize.SMALL} header={emptyHeader} message={emptyMessage} />
      ) : (
        children
      )}
    </Accordion>
  );

  if (!headerAction) return accordion;

  return (
    <Wrapper position="relative" fillWidth>
      {accordion}
      <Wrapper position="absolute" top="8px" right="32px">
        {headerAction}
      </Wrapper>
    </Wrapper>
  );
};

export default PipelineCanvasPanelSection;
