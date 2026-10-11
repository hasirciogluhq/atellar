// Package serverconfig is the API server configuration. `atellar server run` reads
// it from the environment; `atellar server install` renders it into /etc/atellar/api.env.
package serverconfig

import (
	"errors"
	"fmt"
	"net"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/hasirciogluhq/atellar/pkg/pki"
)

const (
	EnvPath           = "/etc/atellar/api.env"
	DefaultMigrations = "/usr/share/atellar/migrations"
)

// Environment variable names.
const (
	EnvDatabaseURL         = "DATABASE_URL"
	EnvMigrationsPath      = "MIGRATIONS_PATH"
	EnvBindAddress         = "BIND_ADDRESS"
	EnvHTTPPort            = "PORT"
	EnvGRPCPort            = "GRPC_PORT"
	EnvClusterCIDR         = "CLUSTER_OVERLAY_CIDR"
	EnvNodeSubnetPrefixLen = "NODE_SUBNET_PREFIX_LEN"
	EnvAdminToken          = "ADMIN_TOKEN"
	EnvNodeStaleAfter      = "NODE_STALE_AFTER"
	EnvPKIDir              = "PKI_DIR"
	EnvTLSSANs             = "TLS_SANS"
)

type Config struct {
	DatabaseURL    string
	MigrationsPath string
	// BindAddress is the listen host; empty listens on every interface.
	BindAddress         string
	HTTPPort            int
	GRPCPort            int
	ClusterCIDR         string
	NodeSubnetPrefixLen int
	// AdminToken guards every operator call. It is required.
	AdminToken     string
	NodeStaleAfter time.Duration
	// PKIDir holds the cluster CA and the server certificate; missing files
	// are created at start (see pkg/pki).
	PKIDir string
	// TLSSANs are extra DNS names or IPs for the server certificate, for
	// clients such as curl that do not verify the fixed name
	// atellar-apiserver.
	TLSSANs []string
}

// Default returns the configuration with every optional field set.
func Default() Config {
	return Config{
		MigrationsPath:      DefaultMigrations,
		HTTPPort:            8080,
		GRPCPort:            9090,
		ClusterCIDR:         "10.0.0.0/8",
		NodeSubnetPrefixLen: 24,
		NodeStaleAfter:      30 * time.Second,
		PKIDir:              pki.DefaultDir,
	}
}

// FromEnv reads the configuration through lookup (os.LookupEnv in
// production).
func FromEnv(lookup func(string) (string, bool)) (Config, error) {
	cfg := Default()
	var errs []error
	str := func(name string, dst *string) {
		if v, ok := lookup(name); ok {
			*dst = strings.TrimSpace(v)
		}
	}
	integer := func(name string, dst *int) {
		if v, ok := lookup(name); ok && strings.TrimSpace(v) != "" {
			n, err := strconv.Atoi(strings.TrimSpace(v))
			if err != nil {
				errs = append(errs, fmt.Errorf("%s: %w", name, err))
				return
			}
			*dst = n
		}
	}
	str(EnvDatabaseURL, &cfg.DatabaseURL)
	str(EnvMigrationsPath, &cfg.MigrationsPath)
	str(EnvBindAddress, &cfg.BindAddress)
	integer(EnvHTTPPort, &cfg.HTTPPort)
	integer(EnvGRPCPort, &cfg.GRPCPort)
	str(EnvClusterCIDR, &cfg.ClusterCIDR)
	integer(EnvNodeSubnetPrefixLen, &cfg.NodeSubnetPrefixLen)
	str(EnvAdminToken, &cfg.AdminToken)
	str(EnvPKIDir, &cfg.PKIDir)
	if v, ok := lookup(EnvTLSSANs); ok {
		cfg.TLSSANs = splitList(v)
	}
	if v, ok := lookup(EnvNodeStaleAfter); ok && strings.TrimSpace(v) != "" {
		d, err := time.ParseDuration(strings.TrimSpace(v))
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", EnvNodeStaleAfter, err))
		} else {
			cfg.NodeStaleAfter = d
		}
	}
	if err := errors.Join(errs...); err != nil {
		return cfg, err
	}
	return cfg, cfg.Validate()
}

// FromOSEnv reads the process environment.
func FromOSEnv() (Config, error) { return FromEnv(os.LookupEnv) }

func (c Config) Validate() error {
	var errs []error
	if c.DatabaseURL == "" {
		errs = append(errs, fmt.Errorf("%s is required", EnvDatabaseURL))
	}
	if c.MigrationsPath == "" {
		errs = append(errs, fmt.Errorf("%s is required", EnvMigrationsPath))
	}
	for name, port := range map[string]int{EnvHTTPPort: c.HTTPPort, EnvGRPCPort: c.GRPCPort} {
		if port <= 0 || port > 65535 {
			errs = append(errs, fmt.Errorf("%s must be between 1 and 65535", name))
		}
	}
	if c.HTTPPort == c.GRPCPort {
		errs = append(errs, fmt.Errorf("%s and %s must differ", EnvHTTPPort, EnvGRPCPort))
	}
	_, cidr, err := net.ParseCIDR(c.ClusterCIDR)
	if err != nil || cidr.IP.To4() == nil {
		errs = append(errs, fmt.Errorf("%s must be an IPv4 CIDR", EnvClusterCIDR))
	} else if ones, _ := cidr.Mask.Size(); c.NodeSubnetPrefixLen < ones || c.NodeSubnetPrefixLen > 30 {
		errs = append(errs, fmt.Errorf("%s must be between %d and 30", EnvNodeSubnetPrefixLen, ones))
	}
	if c.AdminToken == "" {
		errs = append(errs, fmt.Errorf("%s is required", EnvAdminToken))
	}
	if c.PKIDir == "" {
		errs = append(errs, fmt.Errorf("%s is required", EnvPKIDir))
	}
	if c.NodeStaleAfter <= 0 {
		errs = append(errs, fmt.Errorf("%s must be positive", EnvNodeStaleAfter))
	}
	sort.Slice(errs, func(i, j int) bool { return errs[i].Error() < errs[j].Error() })
	return errors.Join(errs...)
}

// HTTPAddr and GRPCAddr are the listen addresses.
func (c Config) HTTPAddr() string { return net.JoinHostPort(c.BindAddress, strconv.Itoa(c.HTTPPort)) }
func (c Config) GRPCAddr() string { return net.JoinHostPort(c.BindAddress, strconv.Itoa(c.GRPCPort)) }

// Env renders the configuration as an environment file.
func (c Config) Env() string {
	lines := []string{
		"# Atellar API server. Generated by atellar server install.",
		EnvDatabaseURL + "=" + c.DatabaseURL,
		EnvMigrationsPath + "=" + c.MigrationsPath,
		EnvBindAddress + "=" + c.BindAddress,
		EnvHTTPPort + "=" + strconv.Itoa(c.HTTPPort),
		EnvGRPCPort + "=" + strconv.Itoa(c.GRPCPort),
		EnvClusterCIDR + "=" + c.ClusterCIDR,
		EnvNodeSubnetPrefixLen + "=" + strconv.Itoa(c.NodeSubnetPrefixLen),
		EnvAdminToken + "=" + c.AdminToken,
		EnvNodeStaleAfter + "=" + c.NodeStaleAfter.String(),
		EnvPKIDir + "=" + c.PKIDir,
		EnvTLSSANs + "=" + strings.Join(c.TLSSANs, ","),
	}
	return strings.Join(lines, "\n") + "\n"
}

func splitList(s string) []string {
	var out []string
	for _, item := range strings.Split(s, ",") {
		if item = strings.TrimSpace(item); item != "" {
			out = append(out, item)
		}
	}
	return out
}

// ParseEnv reads an environment file written by Env.
func ParseEnv(data string) map[string]string {
	out := map[string]string{}
	for _, line := range strings.Split(data, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if k, v, ok := strings.Cut(line, "="); ok {
			out[strings.TrimSpace(k)] = strings.TrimSpace(v)
		}
	}
	return out
}
