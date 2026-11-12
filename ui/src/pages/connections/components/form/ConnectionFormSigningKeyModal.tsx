import { type FC, useState } from "react";

import Button from "@galaxy-io/dls/buttons/Button";
import Alert, { AlertVariant } from "@galaxy-io/dls/feedback/Alert";
import CheckboxInput from "@galaxy-io/dls/inputs/CheckboxInput";
import CopyInput from "@galaxy-io/dls/inputs/CopyInput";
import Flex, { AlignItems, FlexDirection, JustifyContent } from "@galaxy-io/dls/layout/Flex";
import Text from "@galaxy-io/dls/text/Text";
import { FontFamily } from "@galaxy-io/dls/theme/enums";

import ConnectionFormHeader from "@/pages/connections/components/form/ConnectionFormHeader";
import ConnectionFormWrapper from "@/pages/connections/components/form/ConnectionFormWrapper";
import type { ConnectionFormSigningKeyReveal } from "@/pages/connections/components/form/types";

interface ConnectionFormSigningKeyModalProps {
  isOpen: boolean;
  reveal: ConnectionFormSigningKeyReveal;
  onDone: () => void;
}

const ConnectionFormSigningKeyModal: FC<ConnectionFormSigningKeyModalProps> = ({
  isOpen,
  reveal: { connection, signingKey, deliveryUrl },
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
        <Alert
          variant={AlertVariant.WARNING}
          header="Save this key now. It will not be shown again."
        >
          Add it to the sender that signs your deliveries.
        </Alert>
        <CopyInput label="Signing key" value={signingKey} family={FontFamily.MONO} fillWidth />
        {deliveryUrl ? (
          <CopyInput
            label="Delivery URL"
            labelTooltip="Send each delivery to this URL followed by a resource name, such as /orders. Resources are named in the pipeline that reads this source."
            value={deliveryUrl}
            family={FontFamily.MONO}
            fillWidth
          />
        ) : (
          <Text>Delivery URL unavailable. Configure WEBHOOK_PUBLIC_BASE_URL.</Text>
        )}
      </Flex>
    </ConnectionFormWrapper>
  );
};

export default ConnectionFormSigningKeyModal;
