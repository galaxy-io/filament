/** The delivery address for one resource: a full URL when the server knows its public origin, else a path. */
export const webhookDeliveryTarget = (
  sourceId: string,
  resource: string,
  deliveryUrl?: string,
): string => (deliveryUrl ? `${deliveryUrl}/${resource}` : `/hooks/${sourceId}/${resource}`);

export const createWebhookCurlExample = (
  sourceId: string,
  resource: string,
  deliveryUrl?: string,
): string =>
  [
    deliveryUrl
      ? `export FILAMENT_WEBHOOK_URL=${deliveryUrl}/${resource}`
      : `export FILAMENT_WEBHOOK_URL=http://localhost:8080/hooks/${sourceId}/${resource}`,
    "",
    `body='{"event":"created","resource":"${resource}","id":1}'`,
    "timestamp=$(date +%s)",
    'delivery_id="issue-$timestamp-$RANDOM"',
    "signature=$(printf '%s' \"$timestamp.$delivery_id.$body\" | openssl dgst -sha256 -hmac \"$FILAMENT_WEBHOOK_KEY\" -hex | awk '{print $NF}')",
    "",
    'curl -i --request POST "$FILAMENT_WEBHOOK_URL" \\',
    '  --header "Content-Type: application/json" \\',
    '  --header "X-Filament-Delivery-Id: $delivery_id" \\',
    '  --header "X-Filament-Timestamp: $timestamp" \\',
    '  --header "X-Filament-Signature: sha256=$signature" \\',
    '  --data-binary "$body"',
  ].join("\n");
