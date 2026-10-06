import type { ComponentProps, PropsWithChildren, ReactNode } from "react";

import Widget, { WidgetVariant } from "@galaxy-io/dls/widget/Widget";

import EmptyLayout from "@/layouts/EmptyLayout";
import { LayoutSize } from "@/layouts/types";

interface PipelineCanvasPanelSectionProps {
  header: string;
  isEmpty: boolean;
  emptyHeader: string;
  emptyMessage: string;
  padding?: ComponentProps<typeof Widget>["padding"];
  isOpenInitial?: boolean;
  trailing?: ReactNode;
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
  trailing,
  isOpen,
  onToggle,
  children,
}: PropsWithChildren<PipelineCanvasPanelSectionProps>) => {
  return (
    <Widget
      isCollapsible
      header={header}
      variant={WidgetVariant.TERTIARY}
      /* @dls-migrate accordion.padding-other: The body inset follows `size`: remove `padding` (use `isFlush` for 0). */ padding={
        isEmpty ? "24px" : padding
      }
      defaultIsOpen={isOpenInitial}
      isOpen={isOpen}
      onOpenChange={onToggle}
      actions={trailing}
    >
      {isEmpty ? (
        <EmptyLayout size={LayoutSize.SMALL} header={emptyHeader} message={emptyMessage} />
      ) : (
        children
      )}
    </Widget>
  );
};

export default PipelineCanvasPanelSection;
