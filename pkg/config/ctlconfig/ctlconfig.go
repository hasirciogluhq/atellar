// Package ctlconfig is the operator's client configuration, ~/.atellar/config:
// named clusters, contexts that point at them, and the current context.
package ctlconfig

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/hasirciogluhq/atellar/pkg/client"
	"github.com/hasirciogluhq/atellar/pkg/transport"
)

// Cluster is one control plane and how to reach it.
type Cluster struct {
	client.ControlPlane
	// Transport is "grpc" (default) or "http".
	Transport string `json:"transport,omitempty"`
	// Token is the admin token; empty for an open cluster.
	Token string `json:"token,omitempty"`
}

func (c Cluster) Validate() error {
	if err := c.ControlPlane.Validate(); err != nil {
		return err
	}
	_, err := transport.ParseKind(c.Transport)
	return err
}

// Client connects to the cluster.
func (c Cluster) Client() (transport.Client, error) {
	return client.New(client.Config{ControlPlane: c.ControlPlane, Transport: c.Transport, Token: client.Static(c.Token)})
}

type Context struct {
	Cluster string `json:"cluster"`
}

type Config struct {
	CurrentContext string             `json:"current_context,omitempty"`
	Clusters       map[string]Cluster `json:"clusters,omitempty"`
	Contexts       map[string]Context `json:"contexts,omitempty"`
}

// DefaultPath is ~/.atellar/config.
func DefaultPath() string {
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		return filepath.Join(home, ".atellar", "config")
	}
	return filepath.Join(".atellar", "config")
}

// Load returns an empty config when the file does not exist.
func Load(path string) (*Config, error) {
	cfg := &Config{}
	data, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	if err == nil {
		if err := json.Unmarshal(data, cfg); err != nil {
			return nil, fmt.Errorf("parse %s: %w", path, err)
		}
	}
	cfg.init()
	return cfg, nil
}

// Save writes the file with mode 0600: it may hold admin tokens.
func Save(path string, cfg *Config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o600)
}

func (c *Config) init() {
	if c.Clusters == nil {
		c.Clusters = map[string]Cluster{}
	}
	if c.Contexts == nil {
		c.Contexts = map[string]Context{}
	}
}

func (c *Config) SetCluster(name string, cluster Cluster) error {
	if name == "" {
		return errors.New("cluster name is required")
	}
	if err := cluster.Validate(); err != nil {
		return err
	}
	c.init()
	c.Clusters[name] = cluster
	return nil
}

func (c *Config) SetContext(name, cluster string) error {
	if name == "" {
		return errors.New("context name is required")
	}
	c.init()
	if _, ok := c.Clusters[cluster]; !ok {
		return fmt.Errorf("cluster %q not found", cluster)
	}
	c.Contexts[name] = Context{Cluster: cluster}
	return nil
}

func (c *Config) UseContext(name string) error {
	c.init()
	if _, ok := c.Contexts[name]; !ok {
		return fmt.Errorf("context %q not found", name)
	}
	c.CurrentContext = name
	return nil
}

// Current returns the cluster of the current context.
func (c *Config) Current() (Cluster, error) {
	c.init()
	if c.CurrentContext == "" {
		return Cluster{}, errors.New("no current context; run `atellar config use-context NAME`")
	}
	ctx, ok := c.Contexts[c.CurrentContext]
	if !ok {
		return Cluster{}, fmt.Errorf("current context %q not found", c.CurrentContext)
	}
	cluster, ok := c.Clusters[ctx.Cluster]
	if !ok {
		return Cluster{}, fmt.Errorf("cluster %q of context %q not found", ctx.Cluster, c.CurrentContext)
	}
	return cluster, cluster.Validate()
}

// ContextNames returns the context names sorted.
func (c *Config) ContextNames() []string {
	names := make([]string, 0, len(c.Contexts))
	for name := range c.Contexts {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
