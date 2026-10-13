import type { ComponentProps, FC, ReactNode } from "react";

import { ArrowLeftIcon, TrashIcon, XIcon } from "@phosphor-icons/react";

import Button, { ButtonSize, ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import Divider from "@galaxy-io/dls/layout/Divider";
import Flex, { AlignItems } from "@galaxy-io/dls/layout/Flex";
import FlexItem from "@galaxy-io/dls/layout/FlexItem";

import BaseHeader, { BaseHeaderSize } from "@/components/BaseHeader";

interface PipelineCanvasPanelHeaderProps {
  title: string;
  description?: string;
  icon?: ComponentProps<typeof BaseHeader>["icon"];
  tile?: ReactNode;
  actions?: ReactNode[];
  hasDivider?: boolean;
  onBack?: () => void;
  onClose?: () => void;
  onDelete?: () => void;
}

const PipelineCanvasPanelHeader: FC<PipelineCanvasPanelHeaderProps> = ({
  title,
  description,
  icon,
  tile,
  actions,
  hasDivider,
  onBack,
  onClose,
  onDelete,
}) => (
  <>
    <Flex alignItems={AlignItems.CENTER} gap={8} padding={[8, 12]} shrink={0} fillWidth>
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
          actions={[
            ...(actions ?? []),
            onDelete && (
              <Button
                key="delete"
                icon={TrashIcon}
                variant={ButtonVariant.SECONDARY}
                size={ButtonSize.SMALL}
                onClick={onDelete}
                ariaLabel="Delete"
              />
            ),
            onClose && (
              <Button
                key="close"
                icon={XIcon}
                variant={ButtonVariant.SECONDARY}
                size={ButtonSize.SMALL}
                onClick={onClose}
                ariaLabel="Close"
              />
            ),
          ]}
        />
      </FlexItem>
    </Flex>
    {hasDivider && <Divider />}
  </>
);

export default PipelineCanvasPanelHeader;
