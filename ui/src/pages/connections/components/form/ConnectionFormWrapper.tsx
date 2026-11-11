import type { FC, PropsWithChildren, ReactNode } from "react";

import Modal from "@galaxy-io/dls/modal/Modal";

interface ConnectionFormWrapperProps {
  isOpen: boolean;
  header: ReactNode;
  footer?: ReactNode;
  isDismissable?: boolean;
  onClose: () => void;
}

const ConnectionFormWrapper: FC<PropsWithChildren<ConnectionFormWrapperProps>> = ({
  isOpen,
  header,
  footer,
  isDismissable = true,
  onClose,
  children,
}) => (
  <Modal
    isOpen={isOpen}
    header={header}
    footer={footer}
    isDismissable={isDismissable}
    onOpenChange={(isOpen) => {
      if (!isOpen) onClose();
    }}
  >
    {children}
  </Modal>
);

export default ConnectionFormWrapper;
