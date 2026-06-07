package client

import (
	"context"
	"net/http"
	"net/url"
)

func (c *AtellarClient) get(ctx context.Context, path string, query url.Values, out any) error {
	authed := *c
	if authed.Secret != nil {
		if err := authed.EnsureToken(); err == nil {
			if err := authed.Do(ctx, http.MethodGet, path, query, nil, out); err == nil {
				return nil
			}
		}
	}

	plain := *c
	plain.Token = nil
	return plain.Do(ctx, http.MethodGet, path, query, nil, out)
}

func (c *AtellarClient) RegisterNode(ctx context.Context, joinToken string, req RegisterNodeRequest) (*RegisterNodeResult, error) {
	query := url.Values{"token": {joinToken}}
	var result RegisterNodeResult
	if err := c.Do(ctx, http.MethodPost, apiPrefix+"/nodes/register", query, req, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *AtellarClient) RenewNodeAPIKey(ctx context.Context, nodeAPIKey string) (*NodeAPIKeyResult, error) {
	client := c.WithBearer(nodeAPIKey)
	var result NodeAPIKeyResult
	if err := client.Do(ctx, http.MethodPost, apiPrefix+"/nodes/me/api-key/renew", nil, nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *AtellarClient) ListNodes(ctx context.Context) ([]Node, error) {
	var nodes []Node
	if err := c.get(ctx, apiPrefix+"/nodes", nil, &nodes); err != nil {
		return nil, err
	}
	return nodes, nil
}

func (c *AtellarClient) CreateContainer(ctx context.Context, req CreateContainerRequest) (*Container, error) {
	var result Container
	if err := c.Do(ctx, http.MethodPost, apiPrefix+"/containers", nil, req, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *AtellarClient) DeleteContainer(ctx context.Context, containerID string) (*Container, error) {
	var result Container
	if err := c.Do(ctx, http.MethodDelete, apiPrefix+"/containers/"+containerID, nil, nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *AtellarClient) ListContainers(ctx context.Context, nodeID string) ([]Container, error) {
	query := url.Values{}
	if nodeID != "" {
		query.Set("node_id", nodeID)
	}

	var containers []Container
	if err := c.get(ctx, apiPrefix+"/containers", query, &containers); err != nil {
		return nil, err
	}
	return containers, nil
}
