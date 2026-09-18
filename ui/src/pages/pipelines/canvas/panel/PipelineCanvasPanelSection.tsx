import type { ComponentProps, PropsWithChildren, ReactNode } from "react";

import { styled } from "@linaria/react";

import Accordion, { AccordionVariant } from "@galaxy-io/dls/accordion/Accordion";

import EmptyLayout from "@/layouts/EmptyLayout";
import { LayoutSize } from "@/layouts/types";

interface PipelineCanvasPanelSectionProps {
  header: string;
  isEmpty: boolean;
  emptyHeader: string;
  emptyMessage: string;
  padding?: ComponentProps<typeof Accordion>["padding"];
  isOpenInitial?: boolean;
  /** Trailing content in the header, such as an action for the section. */
  metric?: ReactNode;
  /** Interactive header action rendered beside, rather than inside, the accordion button. */
  headerAction?: ReactNode;
  /** Controlled open state; pass with onToggle. */
  isOpen?: boolean;
  onToggle?: () => void;
}

const SectionWrapper = styled.div`
  position: relative;
`;

const HeaderAction = styled.div`
  position: absolute;
  top: 12px;
  right: 32px;
  z-index: 1;
`;

const PipelineCanvasPanelSection = ({
  header,
  isEmpty,
  emptyHeader,
  emptyMessage,
  padding = 0,
  isOpenInitial = true,
  metric,
  headerAction,
  isOpen,
  onToggle,
  children,
}: PropsWithChildren<PipelineCanvasPanelSectionProps>) => (
  <SectionWrapper>
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
        <EmptyLayout size={LayoutSize.SMALL} header={emptyHeader} message={emptyMessage} />
      ) : (
        children
      )}
    </Accordion>
    {headerAction && <HeaderAction>{headerAction}</HeaderAction>}
  </SectionWrapper>
);

export default PipelineCanvasPanelSection;
