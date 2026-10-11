// Package client is the HTTP implementation of api.Interface for callers:
// the atellar CLI and agents that choose the HTTP transport.
package client

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/hasirciogluhq/atellar/pkg/api"
	"github.com/hasirciogluhq/atellar/pkg/api/watch"
	v1 "github.com/hasirciogluhq/atellar/pkg/apis/core/v1"
	"github.com/hasirciogluhq/atellar/pkg/auth/authn"
	"github.com/hasirciogluhq/atellar/pkg/transport/http/wire"
)

const (
	defaultTimeout = 15 * time.Second
	watchBuffer    = 64
	// maxWatchLine bounds one watch frame.
	maxWatchLine = 4 << 20
)

// Client talks to an Atellar API server over HTTP.
type Client struct {
	base  string
	token func() string
	// hc has a request timeout; watches use the same transport without one.
	hc      *http.Client
	watchHC *http.Client
}

var _ api.Interface = (*Client)(nil)

// New returns a client for baseURL ("https://host:port"). token may be nil;
// it is called per request so rotated keys take effect immediately.
// tlsConfig is used for https URLs.
func New(baseURL string, token func() string, tlsConfig *tls.Config) *Client {
	transport := &http.Transport{
		Proxy:               http.ProxyFromEnvironment,
		DialContext:         (&net.Dialer{Timeout: 5 * time.Second, KeepAlive: 30 * time.Second}).DialContext,
		MaxIdleConns:        64,
		MaxIdleConnsPerHost: 16,
		IdleConnTimeout:     90 * time.Second,
		ForceAttemptHTTP2:   true,
		TLSClientConfig:     tlsConfig,
	}
	return &Client{
		base:    strings.TrimRight(baseURL, "/") + wire.Prefix,
		token:   token,
		hc:      &http.Client{Transport: transport, Timeout: defaultTimeout},
		watchHC: &http.Client{Transport: transport},
	}
}

func (c *Client) Nodes() api.NodeInterface           { return nodes{c} }
func (c *Client) Containers() api.ContainerInterface { return containers{c} }
func (c *Client) JoinTokens() api.JoinTokenInterface { return joinTokens{c} }

func (c *Client) newRequest(ctx context.Context, method, path string, query url.Values, body any) (*http.Request, error) {
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reader = bytes.NewReader(data)
	}
	endpoint := c.base + path
	if len(query) > 0 {
		endpoint += "?" + query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, reader)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.token != nil {
		if tok := c.token(); tok != "" {
			req.Header.Set(authn.HeaderAuthorization, authn.Bearer(tok))
		}
	}
	return req, nil
}

func (c *Client) do(ctx context.Context, method, path string, query url.Values, body, out any) error {
	req, err := c.newRequest(ctx, method, path, query, body)
	if err != nil {
		return err
	}
	resp, err := c.hc.Do(req)
	if err != nil {
		return api.Unavailable("%s %s: %v", method, path, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusBadRequest {
		return decodeError(resp)
	}
	if out == nil || resp.StatusCode == http.StatusNoContent {
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return api.Internal("decode %s %s response: %v", method, path, err)
	}
	return nil
}

func decodeError(resp *http.Response) error {
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	var apiErr api.Error
	if json.Unmarshal(data, &apiErr) == nil && apiErr.Message != "" {
		if apiErr.Reason == "" {
			apiErr.Reason = wire.Reason(resp.StatusCode)
		}
		return &apiErr
	}
	return &api.Error{Reason: wire.Reason(resp.StatusCode), Message: fmt.Sprintf("%s: %s", resp.Status, strings.TrimSpace(string(data)))}
}

// watchPath opens a watch stream and decodes it on a goroutine.
func watchPath[T any](ctx context.Context, c *Client, path string, query url.Values) (watch.Interface[T], error) {
	if query == nil {
		query = url.Values{}
	}
	query.Set("watch", "true")
	req, err := c.newRequest(ctx, http.MethodGet, path, query, nil)
	if err != nil {
		return nil, err
	}

	stream := watch.NewStream[T](ctx, watchBuffer)
	req = req.WithContext(stream.Context())
	resp, err := c.watchHC.Do(req)
	if err != nil {
		stream.Stop()
		return nil, api.Unavailable("watch %s: %v", path, err)
	}
	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		stream.Stop()
		return nil, decodeError(resp)
	}

	go func() {
		defer resp.Body.Close()
		scanner := bufio.NewScanner(resp.Body)
		scanner.Buffer(make([]byte, 64<<10), maxWatchLine)
		for scanner.Scan() {
			var frame wire.Event
			if err := json.Unmarshal(scanner.Bytes(), &frame); err != nil {
				stream.Close(api.Internal("watch %s: bad frame: %v", path, err))
				return
			}
			if frame.Type == wire.ErrorEvent {
				var err error = api.Internal("watch %s ended", path)
				if frame.Error != nil {
					err = frame.Error
				}
				stream.Close(err)
				return
			}
			ev := watch.Event[T]{Type: watch.EventType(frame.Type)}
			if len(frame.Object) > 0 {
				if err := json.Unmarshal(frame.Object, &ev.Object); err != nil {
					stream.Close(api.Internal("watch %s: bad object: %v", path, err))
					return
				}
			}
			if !stream.Send(ev) {
				stream.Close(nil)
				return
			}
		}
		err := scanner.Err()
		if stream.Context().Err() != nil {
			err = nil // stopped by the consumer
		} else if err == nil {
			err = io.ErrUnexpectedEOF
		}
		if err != nil && !errors.Is(err, context.Canceled) {
			err = api.Unavailable("watch %s: %v", path, err)
		}
		stream.Close(err)
	}()
	return stream, nil
}

type nodes struct{ c *Client }

func (n nodes) List(ctx context.Context) ([]v1.Node, error) {
	var out []v1.Node
	return out, n.c.do(ctx, http.MethodGet, "/nodes", nil, nil, &out)
}

func (n nodes) Get(ctx context.Context, ref string) (*v1.Node, error) {
	var out v1.Node
	if err := n.c.do(ctx, http.MethodGet, "/nodes/"+url.PathEscape(ref), nil, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (n nodes) Watch(ctx context.Context) (watch.Interface[v1.Node], error) {
	return watchPath[v1.Node](ctx, n.c, "/nodes", nil)
}

func (n nodes) Evict(ctx context.Context, ref string) (*v1.Node, error) {
	var out v1.Node
	if err := n.c.do(ctx, http.MethodPost, "/nodes/"+url.PathEscape(ref)+"/evict", nil, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (n nodes) ListOverlayIPs(ctx context.Context, ref string, freeOnly bool) ([]v1.OverlayIP, error) {
	var query url.Values
	if freeOnly {
		query = url.Values{"free": {"true"}}
	}
	var out []v1.OverlayIP
	return out, n.c.do(ctx, http.MethodGet, "/nodes/"+url.PathEscape(ref)+"/overlay-ips", query, nil, &out)
}

func (n nodes) Register(ctx context.Context, req v1.RegisterNodeRequest) (*v1.RegisterNodeResponse, error) {
	var out v1.RegisterNodeResponse
	if err := n.c.do(ctx, http.MethodPost, "/nodes/register", nil, req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (n nodes) SelfHeartbeat(ctx context.Context) error {
	return n.c.do(ctx, http.MethodPost, "/nodes/self/heartbeat", nil, nil, nil)
}

func (n nodes) SelfReportHardware(ctx context.Context, hw v1.NodeHardware) error {
	return n.c.do(ctx, http.MethodPut, "/nodes/self/hardware", nil, hw, nil)
}

func (n nodes) SelfRenewAPIKey(ctx context.Context) (*v1.NodeAPIKey, error) {
	var out v1.NodeAPIKey
	if err := n.c.do(ctx, http.MethodPost, "/nodes/self/api-key", nil, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

type containers struct{ c *Client }

func listQuery(opts v1.ContainerListOptions) url.Values {
	query := url.Values{}
	if opts.NodeID != "" {
		query.Set("node_id", opts.NodeID)
	}
	if opts.ActiveOnly {
		query.Set("active", "true")
	}
	return query
}

func (cs containers) List(ctx context.Context, opts v1.ContainerListOptions) ([]v1.Container, error) {
	var out []v1.Container
	return out, cs.c.do(ctx, http.MethodGet, "/containers", listQuery(opts), nil, &out)
}

func (cs containers) Get(ctx context.Context, id string) (*v1.Container, error) {
	var out v1.Container
	if err := cs.c.do(ctx, http.MethodGet, "/containers/"+url.PathEscape(id), nil, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (cs containers) Watch(ctx context.Context, opts v1.ContainerListOptions) (watch.Interface[v1.Container], error) {
	return watchPath[v1.Container](ctx, cs.c, "/containers", listQuery(opts))
}

func (cs containers) Create(ctx context.Context, req v1.CreateContainerRequest) (*v1.Container, error) {
	var out v1.Container
	if err := cs.c.do(ctx, http.MethodPost, "/containers", nil, req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (cs containers) Delete(ctx context.Context, id string) (*v1.Container, error) {
	var out v1.Container
	if err := cs.c.do(ctx, http.MethodDelete, "/containers/"+url.PathEscape(id), nil, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (cs containers) ListEvents(ctx context.Context, id string) ([]v1.ContainerEvent, error) {
	var out []v1.ContainerEvent
	return out, cs.c.do(ctx, http.MethodGet, "/containers/"+url.PathEscape(id)+"/events", nil, nil, &out)
}

func (cs containers) ReportRuntime(ctx context.Context, id string, report v1.ContainerRuntimeReport) (*v1.Container, error) {
	var out v1.Container
	if err := cs.c.do(ctx, http.MethodPut, "/containers/"+url.PathEscape(id)+"/runtime", nil, report, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (cs containers) AllocateOverlayIP(ctx context.Context, id string) (*v1.Container, error) {
	var out v1.Container
	if err := cs.c.do(ctx, http.MethodPost, "/containers/"+url.PathEscape(id)+"/overlay-ip", nil, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

type joinTokens struct{ c *Client }

func (j joinTokens) List(ctx context.Context) ([]v1.JoinToken, error) {
	var out []v1.JoinToken
	return out, j.c.do(ctx, http.MethodGet, "/jointokens", nil, nil, &out)
}

func (j joinTokens) Create(ctx context.Context, req v1.CreateJoinTokenRequest) (*v1.JoinToken, error) {
	var out v1.JoinToken
	if err := j.c.do(ctx, http.MethodPost, "/jointokens", nil, req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Close releases idle connections. The client stays usable.
func (c *Client) Close() error {
	c.hc.CloseIdleConnections()
	return nil
}
