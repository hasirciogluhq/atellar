package controlplane

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/hasirciogluhq/atellar/internal/modules/nodes/domain/node"
)

type Client struct {
	baseURL    string
	httpClient *http.Client
}

func NewClient(baseURL string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

type RegisterRequest struct {
	Name           string `json:"name,omitempty"`
	PublicIP       string `json:"public_ip,omitempty"`
	PrivateIP      string `json:"private_ip,omitempty"`
	AgentVersion   string `json:"agent_version,omitempty"`
	ContainerdSock string `json:"containerd_sock,omitempty"`
}

func (c *Client) Register(ctx context.Context, joinToken string, req RegisterRequest) (*node.NodeEntity, error) {
	endpoint := fmt.Sprintf("%s/api/v1/nodes/register?token=%s", c.baseURL, joinToken)

	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= http.StatusBadRequest {
		return nil, fmt.Errorf("register failed (%d): %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}

	var registeredNode node.NodeEntity
	if err := json.Unmarshal(respBody, &registeredNode); err != nil {
		return nil, err
	}

	return &registeredNode, nil
}

func (c *Client) SendHeartbeat(ctx context.Context, nodeID string) error {
	endpoint := fmt.Sprintf("%s/api/v1/nodes/%s/heartbeat", c.baseURL, nodeID)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, nil)
	if err != nil {
		return err
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusBadRequest {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("heartbeat failed (%d): %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}

	return nil
}
