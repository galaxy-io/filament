export enum WebhookEndpointStatus {
  NO_KEY = "NO_KEY",
  NO_PIPELINE = "NO_PIPELINE",
  PIPELINE_DELETED = "PIPELINE_DELETED",
  OFF = "OFF",
  NOT_RUNNING = "NOT_RUNNING",
  WAITING = "WAITING",
  LIVE = "LIVE",
}

export enum WebhookIngestion {
  API = "api",
  WEBHOOKS = "webhooks",
  BOTH = "both",
}
