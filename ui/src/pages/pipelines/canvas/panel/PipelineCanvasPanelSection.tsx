import type { ComponentProps, PropsWithChildren, ReactNode } from "react";

import type { Icon as PhosphorIcon } from "@phosphor-icons/react";

import Accordion, { AccordionVariant } from "@galaxy-io/dls/accordion/Accordion";

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
  trailing?: ReactNode;
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
  trailing,
  isOpen,
  onToggle,
  children,
}: PropsWithChildren<PipelineCanvasPanelSectionProps>) => {
  return (
    <Accordion
      icon={icon}
      header={header}
      variant={AccordionVariant.TERTIARY}
      padding={isEmpty ? "24px" : padding}
      isOpenInitial={isOpenInitial}
      isOpen={isOpen}
      onToggle={onToggle}
      trailing={trailing}
    >
      {isEmpty ? (
        <EmptyLayout size={LayoutSize.SMALL} header={emptyHeader} message={emptyMessage} />
      ) : (
        children
      )}
    </Accordion>
  );
};

export default PipelineCanvasPanelSection;
