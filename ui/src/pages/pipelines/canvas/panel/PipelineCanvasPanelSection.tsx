import type { ComponentProps, PropsWithChildren, ReactNode } from "react";

import type { Icon as PhosphorIcon } from "@phosphor-icons/react";

import Accordion, { AccordionVariant } from "@galaxy-io/dls/accordion/Accordion";
import FlexItem from "@galaxy-io/dls/containers/FlexItem";
import Wrapper from "@galaxy-io/dls/containers/Wrapper";

import EmptyLayout from "@/layouts/EmptyLayout";
import PendingLayout from "@/layouts/PendingLayout";
import { LayoutSize } from "@/layouts/types";

const PIPELINE_CANVAS_PANEL_SECTION_EMPTY_HEIGHT = 37;

interface PipelineCanvasPanelSectionProps {
  icon?: PhosphorIcon;
  header: string;
  isEmpty: boolean;
  /** Renders a pending state the same size as the empty state. */
  isPending?: boolean;
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
  isPending = false,
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
  const body = isPending ? (
    <FlexItem height={PIPELINE_CANVAS_PANEL_SECTION_EMPTY_HEIGHT} fillWidth>
      <PendingLayout size={LayoutSize.SMALL} />
    </FlexItem>
  ) : isEmpty ? (
    <EmptyLayout size={LayoutSize.SMALL} header={emptyHeader} message={emptyMessage} />
  ) : (
    children
  );

  const accordion = (
    <Accordion
      icon={icon}
      header={header}
      variant={AccordionVariant.TERTIARY}
      padding={isEmpty || isPending ? "24px" : padding}
      isOpenInitial={isOpenInitial}
      isOpen={isOpen}
      onToggle={onToggle}
      metric={headerAction ? <FlexItem width={headerActionWidth} shrink={0} /> : metric}
    >
      {body}
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
