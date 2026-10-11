// Package server assembles the API server process: storage, the API, both
// network transports and the node lifecycle controller.
package server

import (
	"context"
	"fmt"
	"log"
	"net"
	"sync"

	"github.com/hasirciogluhq/atellar/pkg/apiserver"
	"github.com/hasirciogluhq/atellar/pkg/auth"
	"github.com/hasirciogluhq/atellar/pkg/config/serverconfig"
	"github.com/hasirciogluhq/atellar/pkg/controller/nodelifecycle"
	"github.com/hasirciogluhq/atellar/pkg/pki"
	"github.com/hasirciogluhq/atellar/pkg/storage"
	"github.com/hasirciogluhq/atellar/pkg/transport"
)

// Listeners lets tests pass pre-bound sockets; nil fields listen on the
// configured ports.
type Listeners struct {
	HTTP net.Listener
	GRPC net.Listener
}

// Run serves until ctx is done or a listener fails.
func Run(ctx context.Context, cfg serverconfig.Config, store storage.Interface, ls Listeners) error {
	api, err := apiserver.New(apiserver.Config{
		Store:               store,
		ClusterCIDR:         cfg.ClusterCIDR,
		NodeSubnetPrefixLen: cfg.NodeSubnetPrefixLen,
	})
	if err != nil {
		return err
	}
	if cfg.AdminToken == "" {
		return fmt.Errorf("%s is required", serverconfig.EnvAdminToken)
	}
	authn := auth.NewTokenAuthenticator(cfg.AdminToken, store.GetNodeByAPIKeyHash)
	certs, err := pki.LoadOrCreateServer(cfg.PKIDir, cfg.TLSSANs)
	if err != nil {
		return fmt.Errorf("pki: %w", err)
	}
	log.Printf("atellar server: cluster CA %s (%s)", certs.CAHash(), cfg.PKIDir)

	if ls.HTTP == nil {
		if ls.HTTP, err = net.Listen("tcp", cfg.HTTPAddr()); err != nil {
			return fmt.Errorf("http listen: %w", err)
		}
	}
	if ls.GRPC == nil {
		if ls.GRPC, err = net.Listen("tcp", cfg.GRPCAddr()); err != nil {
			ls.HTTP.Close()
			return fmt.Errorf("grpc listen: %w", err)
		}
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	errs := make(chan error, 2)
	var wg sync.WaitGroup
	for _, s := range []struct {
		kind transport.Kind
		ln   net.Listener
	}{{transport.HTTP, ls.HTTP}, {transport.GRPC, ls.GRPC}} {
		srv, err := transport.NewServer(s.kind, api, authn, transport.WithTLS(certs.TLSConfig()))
		if err != nil {
			return err
		}
		log.Printf("atellar server: serving %s on %s", s.kind, s.ln.Addr())
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := srv.Serve(ctx, s.ln); err != nil && ctx.Err() == nil {
				errs <- fmt.Errorf("%s server: %w", s.kind, err)
				cancel()
			}
		}()
	}

	lifecycle := nodelifecycle.New(store, api.NodeEvents(), cfg.NodeStaleAfter, 0)
	wg.Add(1)
	go func() { defer wg.Done(); lifecycle.Run(ctx) }()

	<-ctx.Done()
	wg.Wait()
	select {
	case err := <-errs:
		return err
	default:
		return nil
	}
}
