package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

const (
	DefaultDirName  = ".atellar"
	DefaultFileName = "config"
)

type Cluster struct {
	Address  string `json:"address"`
	HTTPPort int    `json:"http_port"`
	GRPCPort int    `json:"grpc_port"`
}

type Context struct {
	Cluster string `json:"cluster"`
}

type Config struct {
	CurrentContext string             `json:"current_context,omitempty"`
	Clusters       map[string]Cluster `json:"clusters,omitempty"`
	Contexts       map[string]Context `json:"contexts,omitempty"`
}

func DefaultPath() string {
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		return filepath.Join(home, DefaultDirName, DefaultFileName)
	}
	return filepath.Join(".", DefaultDirName, DefaultFileName)
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return newConfig(), nil
		}
		return nil, err
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	cfg.ensureMaps()
	return &cfg, nil
}

func Save(path string, cfg *Config) error {
	cfg.ensureMaps()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0o600)
}

func (c *Config) SetCluster(name string, cluster Cluster) error {
	if name == "" {
		return fmt.Errorf("cluster name is required")
	}
	if cluster.Address == "" {
		return fmt.Errorf("control plane address is required")
	}
	if cluster.HTTPPort <= 0 {
		return fmt.Errorf("http port is required")
	}
	if cluster.GRPCPort <= 0 {
		return fmt.Errorf("grpc port is required")
	}

	c.ensureMaps()
	c.Clusters[name] = cluster
	return nil
}

func (c *Config) SetContext(name, clusterName string) error {
	if name == "" {
		return fmt.Errorf("context name is required")
	}
	if clusterName == "" {
		return fmt.Errorf("cluster name is required")
	}

	c.ensureMaps()
	if _, ok := c.Clusters[clusterName]; !ok {
		return fmt.Errorf("cluster %q not found", clusterName)
	}

	c.Contexts[name] = Context{Cluster: clusterName}
	return nil
}

func (c *Config) UseContext(name string) error {
	if name == "" {
		return fmt.Errorf("context name is required")
	}
	c.ensureMaps()
	if _, ok := c.Contexts[name]; !ok {
		return fmt.Errorf("context %q not found", name)
	}
	c.CurrentContext = name
	return nil
}

func (c *Config) ResolveCurrentCluster() (Cluster, error) {
	c.ensureMaps()
	if c.CurrentContext == "" {
		return Cluster{}, fmt.Errorf("current context is not set")
	}

	ctx, ok := c.Contexts[c.CurrentContext]
	if !ok {
		return Cluster{}, fmt.Errorf("current context %q not found", c.CurrentContext)
	}

	cluster, ok := c.Clusters[ctx.Cluster]
	if !ok {
		return Cluster{}, fmt.Errorf("cluster %q not found", ctx.Cluster)
	}

	if cluster.Address == "" || cluster.HTTPPort <= 0 || cluster.GRPCPort <= 0 {
		return Cluster{}, fmt.Errorf("invalid current context %q", c.CurrentContext)
	}
	return cluster, nil
}

func (c *Config) ContextNames() []string {
	c.ensureMaps()
	names := make([]string, 0, len(c.Contexts))
	for name := range c.Contexts {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func newConfig() *Config {
	cfg := &Config{}
	cfg.ensureMaps()
	return cfg
}

func (c *Config) ensureMaps() {
	if c.Clusters == nil {
		c.Clusters = make(map[string]Cluster)
	}
	if c.Contexts == nil {
		c.Contexts = make(map[string]Context)
	}
}
