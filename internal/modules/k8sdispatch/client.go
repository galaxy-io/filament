package k8sdispatch

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

type client struct {
	baseURL string
	token   string
	http    *http.Client
}

func newClient(cfg Config) (*client, error) {
// TODO: Leon Creating a client to communicate with k8s 
}

func (c *client) createJob(ctx context.Context, namespace string, j job) error {
// TODO: Leon Create a job using the client given the job spec from job.go
}
