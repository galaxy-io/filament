import { type FC, useState } from "react";

import Button, { ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import PasswordInput from "@galaxy-io/dls/inputs/PasswordInput";
import Flex, { AlignItems, FlexDirection, JustifyContent } from "@galaxy-io/dls/layout/Flex";
import Modal from "@galaxy-io/dls/modal/Modal";
import Text, { TextSize, TextVariant } from "@galaxy-io/dls/text/Text";

import { WEBHOOK_SIGNING_SECRET_MIN_LENGTH } from "@/components/webhooks/constants";

interface ConnectionDrawerWebhookSigningKeyPasteModalProps {
  isOpen: boolean;
  hasKey: boolean;
  isPending: boolean;
  onSave: (secret: string) => void;
  onClose: () => void;
}

interface ConnectionDrawerWebhookSigningKeyPasteModalState {
  secret: string;
  shouldShowError: boolean;
}

const DEFAULT_STATE: ConnectionDrawerWebhookSigningKeyPasteModalState = {
  secret: "",
  shouldShowError: false,
};

const ConnectionDrawerWebhookSigningKeyPasteModal: FC<
  ConnectionDrawerWebhookSigningKeyPasteModalProps
> = ({ isOpen, hasKey, isPending, onSave, onClose }) => {
  const [state, setState] =
    useState<ConnectionDrawerWebhookSigningKeyPasteModalState>(DEFAULT_STATE);
  const isValid = state.secret.length >= WEBHOOK_SIGNING_SECRET_MIN_LENGTH;
  const error =
    state.shouldShowError && !isValid
      ? `Use at least ${WEBHOOK_SIGNING_SECRET_MIN_LENGTH} characters`
      : undefined;

  const handleSecretChange = (secret: string) => {
    setState({ secret, shouldShowError: false });
  };

  const handleSave = () => {
    if (!isValid) {
      setState((prev) => ({ ...prev, shouldShowError: true }));
      return;
    }
    onSave(state.secret);
  };

  return (
    <Modal
      isOpen={isOpen}
      header={hasKey ? "Replace signing secret" : "Add signing secret"}
      footer={
        <Flex alignItems={AlignItems.CENTER} justifyContent={JustifyContent.END} gap={8} fillWidth>
          <Button label="Cancel" variant={ButtonVariant.SECONDARY} onClick={onClose} />
          <Button label="Save secret" isLoading={isPending} onClick={handleSave} />
        </Flex>
      }
      onOpenChange={(isOpen) => {
        if (!isOpen) onClose();
      }}
    >
      <Flex alignItems={AlignItems.STRETCH} direction={FlexDirection.COLUMN} gap={12}>
        <Text size={TextSize.BODY_MD} variant={TextVariant.SECONDARY}>
          {hasKey
            ? "Use the secret the provider shows for this webhook. The current key keeps working for 72 hours."
            : "Use the secret the provider shows for this webhook."}
        </Text>
        <PasswordInput
          label="Secret"
          value={state.secret}
          onChange={handleSecretChange}
          placeholder="Paste the secret..."
          error={error}
          isRequired
          isDisabled={isPending}
          fillWidth
          autoFocus
        />
      </Flex>
    </Modal>
  );
};

export default ConnectionDrawerWebhookSigningKeyPasteModal;
