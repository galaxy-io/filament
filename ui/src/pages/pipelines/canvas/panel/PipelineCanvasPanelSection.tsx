import type { ComponentProps, PropsWithChildren, ReactNode } from "react";

import type { Icon as PhosphorIcon } from "@phosphor-icons/react";

import Accordion, { AccordionVariant } from "@galaxy-io/dls/accordion/Accordion";
import FlexItem from "@galaxy-io/dls/containers/FlexItem";
import Wrapper from "@galaxy-io/dls/containers/Wrapper";

import EmptyLayout from "@/layouts/EmptyLayout";
import { LayoutSize } from "@/layouts/types";

interface PipelineCanvasPanelSectionProps {
  icon?: PhosphorIcon;
  header: string;
  isEmpty: boolean;
  emptyHeader: string;
  emptyMessage: string;
  padding?: ComponentProps<typeof Accordion>["padding"];
  isOpenInitial?: boolean;
  /** Trailing content in the header, such as an action for the section. */
  metric?: ReactNode;
  /** Independent action visually placed in the header, outside its toggle button. */
  headerAction?: ReactNode;
  /** Space reserved inside the header for headerAction. */
  headerActionWidth?: number;
  /** Controlled open state; pass with onToggle. */
  isOpen?: boolean;
  onToggle?: () => void;
}

const PipelineCanvasPanelSection = ({
  icon,
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
      icon={icon}
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

  // Accordion's metric is deliberately non-interactive because DLS renders it
  // inside the toggle button. Overlay the sibling action into the reserved
  // header space so the controls remain visually grouped and semantically independent.
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
