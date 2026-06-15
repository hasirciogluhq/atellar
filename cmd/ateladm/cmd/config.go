package cmd

import (
	"fmt"

	atelconfig "github.com/hasirciogluhq/atellar/internal/cli/atelctl/config"
	"github.com/hasirciogluhq/atellar/pkg/client"
)

var configPath string

func resolveControlPlaneFromFlagsOrConfig(address string, httpPort, grpcPort int) (client.ControlPlane, error) {
	if address != "" || httpPort != 0 || grpcPort != 0 {
		cp := client.ControlPlane{Address: address, HTTPPort: httpPort, GRPCPort: grpcPort}
		if err := cp.Validate(); err != nil {
			return client.ControlPlane{}, err
		}
		return cp, nil
	}

	cfg, err := atelconfig.Load(configPath)
	if err != nil {
		return client.ControlPlane{}, err
	}

	cluster, err := cfg.ResolveCurrentCluster()
	if err != nil {
		return client.ControlPlane{}, fmt.Errorf("%w; run `atelctl config use-context <name>` or pass endpoint flags", err)
	}

	return client.ControlPlane{
		Address:  cluster.Address,
		HTTPPort: cluster.HTTPPort,
		GRPCPort: cluster.GRPCPort,
	}, nil
}

func init() {
	rootCmd.PersistentFlags().StringVar(&configPath, "config", atelconfig.DefaultPath(), "atelctl config file")
}
