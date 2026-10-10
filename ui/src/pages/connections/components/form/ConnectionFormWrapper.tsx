import type { FC, PropsWithChildren, ReactNode } from "react";

import Modal, { type ModalSize } from "@galaxy-io/dls/modal/Modal";

interface ConnectionFormWrapperProps {
  size: ModalSize;
  header: ReactNode;
  footer?: ReactNode;
  onClose: () => void;
}

const ConnectionFormWrapper: FC<PropsWithChildren<ConnectionFormWrapperProps>> = ({
  size,
  header,
  footer,
  onClose,
  children,
}) => (
  <Modal
    isOpen
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
