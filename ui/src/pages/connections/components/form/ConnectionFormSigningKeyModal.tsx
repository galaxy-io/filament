import { type FC, useState } from "react";

import Button from "@galaxy-io/dls/buttons/Button";
import Alert, { AlertVariant } from "@galaxy-io/dls/feedback/Alert";
import CheckboxInput from "@galaxy-io/dls/inputs/CheckboxInput";
import CopyInput from "@galaxy-io/dls/inputs/CopyInput";
import Flex, { AlignItems, FlexDirection, JustifyContent } from "@galaxy-io/dls/layout/Flex";
import { FontFamily } from "@galaxy-io/dls/theme/enums";

import ConnectionFormHeader from "@/pages/connections/components/form/ConnectionFormHeader";
import ConnectionFormWebhookDeliveryTargets from "@/pages/connections/components/form/ConnectionFormWebhookDeliveryTargets";
import ConnectionFormWebhookSetupGuide from "@/pages/connections/components/form/ConnectionFormWebhookSetupGuide";
import ConnectionFormWrapper from "@/pages/connections/components/form/ConnectionFormWrapper";
import type { ConnectionFormSigningKeyReveal } from "@/pages/connections/components/form/types";

interface ConnectionFormSigningKeyModalProps {
  isOpen: boolean;
  reveal: ConnectionFormSigningKeyReveal;
  onDone: () => void;
}

const ConnectionFormSigningKeyModal: FC<ConnectionFormSigningKeyModalProps> = ({
  isOpen,
  reveal: { connection, webhook, signingKey, deliveryUrl },
  onDone,
}) => {
  const [isStored, setIsStored] = useState(false);

  return (
    <ConnectionFormWrapper
      isOpen={isOpen}
      isDismissable={isStored}
      header={
        <ConnectionFormHeader
          connectorName={connection.connector}
          connectorKind={connection.kind}
          title={`${connection.name} signing key`}
        />
      }
      footer={
        <Flex
          alignItems={AlignItems.CENTER}
          justifyContent={JustifyContent.SPACE_BETWEEN}
          fillWidth
        >
          <CheckboxInput label="I stored this key" isChecked={isStored} onChange={setIsStored} />
          <Button label="Done" isDisabled={!isStored} onClick={onDone} />
        </Flex>
      }
      onClose={onDone}
    >
      <Flex alignItems={AlignItems.STRETCH} direction={FlexDirection.COLUMN} gap={16}>
        <Alert variant={AlertVariant.WARNING} header="This key is shown only once">
          Copy it into the sender before closing.
        </Alert>
        <CopyInput label="Signing key" value={signingKey} family={FontFamily.MONO} fillWidth />
        <ConnectionFormWebhookDeliveryTargets webhook={webhook} deliveryUrl={deliveryUrl} />
        <ConnectionFormWebhookSetupGuide webhook={webhook} deliveryUrl={deliveryUrl} />
      </Flex>
    </ConnectionFormWrapper>
  );
};

export default ConnectionFormSigningKeyModal;
