import type { ComponentProps } from "react";

import { ArrowLeftIcon } from "@phosphor-icons/react";

import Button, { ButtonSize, ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import FlexItem from "@galaxy-io/dls/containers/FlexItem";
import FlexWrapper, { AlignItems } from "@galaxy-io/dls/containers/FlexWrapper";
import HorizontalDivider from "@galaxy-io/dls/dividers/HorizontalDivider";

import BaseHeader, { BaseHeaderSize } from "@/layouts/components/BaseHeader";

interface PipelineCanvasPanelHeaderProps {
  title: string;
  description?: string;
  icon?: ComponentProps<typeof BaseHeader>["icon"];
  tile?: React.ReactNode;
  onBack?: () => void;
  onClose: () => void;
}

const PipelineCanvasPanelHeader = ({
  title,
  description,
  icon,
  tile,
  onBack,
  onClose,
}: PipelineCanvasPanelHeaderProps) => (
  <>
    <FlexWrapper alignItems={AlignItems.CENTER} gap={8} padding="8px 12px" shrink={0} fillWidth>
      {onBack && (
        <FlexItem shrink={0}>
          <Button
            icon={ArrowLeftIcon}
            variant={ButtonVariant.SECONDARY}
            size={ButtonSize.SMALL}
            onClick={onBack}
            ariaLabel="Back to configuration"
          />
        </FlexItem>
      )}
      {tile && <FlexItem shrink={0}>{tile}</FlexItem>}
      <FlexItem grow={1} minWidth={0}>
        <BaseHeader
          size={BaseHeaderSize.SMALL}
          title={title}
          description={description}
          icon={icon}
          onClose={onClose}
        />
      </FlexItem>
    </FlexWrapper>
    <HorizontalDivider />
  </>
);

export default PipelineCanvasPanelHeader;
