import type { FC, PropsWithChildren, ReactNode } from "react";

import Modal, { type ModalSize } from "@galaxy-io/dls/modal/Modal";

interface ConnectionFormWrapperProps {
  isOpen: boolean;
  size: ModalSize;
  header: ReactNode;
  footer?: ReactNode;
  onClose: () => void;
}

const ConnectionFormWrapper: FC<PropsWithChildren<ConnectionFormWrapperProps>> = ({
  isOpen,
  size,
  header,
  footer,
  onClose,
  children,
}) => (
  <Modal
    isOpen={isOpen}
    size={size}
    header={header}
    footer={footer}
    onOpenChange={(isOpen) => {
      if (!isOpen) onClose();
    }}
  >
    {children}
  </Modal>
);

export default ConnectionFormWrapper;
