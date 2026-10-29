import type { FC, PropsWithChildren, ReactNode } from "react";

import Modal from "@galaxy-io/dls/modal/Modal";

interface ConnectionFormWrapperProps {
  isOpen: boolean;
  header: ReactNode;
  footer?: ReactNode;
  onClose: () => void;
}

const ConnectionFormWrapper: FC<PropsWithChildren<ConnectionFormWrapperProps>> = ({
  isOpen,
  header,
  footer,
  onClose,
  children,
}) => (
  <Modal
    isOpen={isOpen}
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
