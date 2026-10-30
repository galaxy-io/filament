export const PIPELINE_WORKER_CONFIGURATION_DEFAULT_TEXT = `{
  "resources": {
    "requests": { "cpu": "500m", "memory": "256Mi" },
    "limits": { "cpu": "1000m", "memory": "512Mi" }
  },
  "nodeSelector": {},
  "tolerations": []
}`;
