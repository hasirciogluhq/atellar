package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	defaultTimeout     = 15 * time.Second
	apiPrefix          = "/api/v1"
	bearerPrefix       = "Bearer "
	serviceSecretPath  = "/var/run/secrets/atellar/service-account/secret"
	envServiceSecret   = "ATELLAR_SERVICE_ACCOUNT_SECRET"
	envServiceToken    = "ATELLAR_SERVICE_ACCOUNT_TOKEN"
)

type AtellarClient struct {
	BaseURL    string
	Token      *string
	Secret     *string
	httpClient *http.Client
}

type Options struct {
	BaseURL string
	Token   *string
	Secret  *string
}

func New(opts Options) *AtellarClient {
	resolvedToken := opts.Token
	resolvedSecret := opts.Secret

	if resolvedSecret == nil {
		if data, err := os.ReadFile(serviceSecretPath); err == nil {
			secret := strings.TrimSpace(string(data))
			resolvedSecret = &secret
		} else if envSecret := os.Getenv(envServiceSecret); envSecret != "" {
			resolvedSecret = &envSecret
		}
	}

	if resolvedToken == nil {
		if envToken := os.Getenv(envServiceToken); envToken != "" {
			resolvedToken = &envToken
		}
	}

	return &AtellarClient{
		BaseURL: strings.TrimRight(opts.BaseURL, "/"),
		Token:   resolvedToken,
		Secret:  resolvedSecret,
		httpClient: &http.Client{
			Timeout: defaultTimeout,
		},
	}
}

// NewAtellarClient keeps the original constructor name.
func NewAtellarClient(opts Options) *AtellarClient {
	return New(opts)
}

func (c *AtellarClient) WithBaseURL(baseURL string) *AtellarClient {
	clone := *c
	clone.BaseURL = strings.TrimRight(baseURL, "/")
	return &clone
}

func (c *AtellarClient) WithBearer(token string) *AtellarClient {
	clone := *c
	t := token
	clone.Token = &t
	return &clone
}

func (c *AtellarClient) EnsureToken() error {
	if c.Token != nil && *c.Token != "" {
		parsed, err := jwt.Parse(*c.Token, func(*jwt.Token) (any, error) {
			if c.Secret == nil {
				return nil, errors.New("secret required to validate token")
			}
			return []byte(*c.Secret), nil
		})
		if err == nil && parsed.Valid {
			if exp, err := parsed.Claims.GetExpirationTime(); err == nil {
				if exp.After(time.Now().Add(3 * time.Second)) {
					return nil
				}
			} else {
				return nil
			}
		}
		c.Token = nil
	}

	if c.Secret == nil {
		return errors.New("secret required for token generation")
	}

	claims := jwt.MapClaims{
		"exp": time.Now().Add(time.Hour).Unix(),
		"iat": time.Now().Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	signed, err := token.SignedString([]byte(*c.Secret))
	if err != nil {
		return err
	}

	c.Token = &signed
	return nil
}

func (c *AtellarClient) Do(ctx context.Context, method, path string, query url.Values, body any, out any) error {
	if c.BaseURL == "" {
		return errors.New("base url is required")
	}

	var bodyReader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return err
		}
		bodyReader = bytes.NewReader(encoded)
	}

	endpoint := c.BaseURL + path
	if len(query) > 0 {
		endpoint += "?" + query.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, method, endpoint, bodyReader)
	if err != nil {
		return err
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	if c.Token != nil && *c.Token != "" {
		req.Header.Set("Authorization", bearerPrefix+*c.Token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if resp.StatusCode >= http.StatusBadRequest {
		return fmt.Errorf("%s %s failed (%d): %s", method, path, resp.StatusCode, strings.TrimSpace(string(respBody)))
	}

	if out == nil || len(respBody) == 0 {
		return nil
	}

	return json.Unmarshal(respBody, out)
}

func (c *AtellarClient) DoJSON(ctx context.Context, method, path string, body any, out any) error {
	return c.Do(ctx, method, path, nil, body, out)
}
