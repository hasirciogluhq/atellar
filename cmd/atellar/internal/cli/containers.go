package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	v1 "github.com/hasirciogluhq/atellar/pkg/apis/core/v1"
)

var containerHeader = []string{"ID", "NODE", "IMAGE", "STATUS", "OVERLAY_IP", "RESTARTS", "ERROR"}

func containerRow(c v1.Container) []string {
	return []string{c.ID, c.NodeID, c.Image, string(c.Status), or(c.OverlayIP, "-"), fmt.Sprint(c.RestartCount), str(c.ErrorMessage)}
}

func (e *env) containersCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "containers", Aliases: []string{"container", "ctr"}, Short: "Deploy and manage containers"}

	var opts v1.ContainerListOptions
	list := &cobra.Command{
		Use:   "list",
		Short: "List containers",
		Args:  cobra.NoArgs,
		RunE: func(c *cobra.Command, _ []string) error {
			cl, err := e.connect()
			if err != nil {
				return err
			}
			defer cl.Close()
			items, err := cl.Containers().List(c.Context(), opts)
			if err != nil {
				return err
			}
			rows := make([][]string, 0, len(items))
			for _, it := range items {
				rows = append(rows, containerRow(it))
			}
			return e.print(items, containerHeader, rows)
		},
	}
	list.Flags().StringVar(&opts.NodeID, "node", "", "only containers on this node ID")
	list.Flags().BoolVar(&opts.ActiveOnly, "active", false, "hide terminated containers")

	get := e.containerByID("get", "Show one container", func(c *cobra.Command, id string) (*v1.Container, error) {
		cl, err := e.connect()
		if err != nil {
			return nil, err
		}
		defer cl.Close()
		return cl.Containers().Get(c.Context(), id)
	})
	del := e.containerByID("delete", "Stop and remove a container", func(c *cobra.Command, id string) (*v1.Container, error) {
		cl, err := e.connect()
		if err != nil {
			return nil, err
		}
		defer cl.Close()
		return cl.Containers().Delete(c.Context(), id)
	})

	var (
		req     v1.CreateContainerRequest
		env     []string
		command string
		cpu     float64
		memory  int32
		policy  string
	)
	create := &cobra.Command{
		Use:   "create",
		Short: "Deploy a container; the scheduler picks the node",
		Args:  cobra.NoArgs,
		RunE: func(c *cobra.Command, _ []string) error {
			req.RestartPolicy = v1.RestartPolicy(policy)
			if command != "" {
				req.Command = strings.Fields(command)
			}
			if len(env) > 0 {
				req.Env = map[string]string{}
				for _, kv := range env {
					k, v, ok := strings.Cut(kv, "=")
					if !ok || k == "" {
						return fmt.Errorf("--env %q: want KEY=VALUE", kv)
					}
					req.Env[k] = v
				}
			}
			if c.Flags().Changed("cpu") {
				req.CpuLimit = &cpu
			}
			if c.Flags().Changed("memory") {
				req.MemoryLimitMiB = &memory
			}
			cl, err := e.connect()
			if err != nil {
				return err
			}
			defer cl.Close()
			created, err := cl.Containers().Create(c.Context(), req)
			if err != nil {
				return err
			}
			return e.print(created, containerHeader, [][]string{containerRow(*created)})
		},
	}
	f := create.Flags()
	f.StringVar(&req.Image, "image", "", "image reference, e.g. docker.io/library/nginx:1.27")
	f.StringVar(&command, "command", "", "arguments that replace the image CMD, split on spaces")
	f.StringArrayVar(&env, "env", nil, "environment variable KEY=VALUE (repeatable)")
	f.Float64Var(&cpu, "cpu", 0, "CPU limit in cores, e.g. 0.5")
	f.Int32Var(&memory, "memory", 0, "memory limit in MiB")
	f.StringVar(&policy, "restart", "on-failure", "restart policy: no, on-failure or always")
	_ = create.MarkFlagRequired("image")

	events := &cobra.Command{
		Use:   "events ID",
		Short: "Show a container's lifecycle events, newest first",
		Args:  cobra.ExactArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			cl, err := e.connect()
			if err != nil {
				return err
			}
			defer cl.Close()
			items, err := cl.Containers().ListEvents(c.Context(), args[0])
			if err != nil {
				return err
			}
			rows := make([][]string, 0, len(items))
			for _, ev := range items {
				rows = append(rows, []string{ev.CreatedAt.Format("2006-01-02T15:04:05.000Z07:00"), string(ev.Event), str(ev.Message)})
			}
			return e.print(items, []string{"TIME", "EVENT", "MESSAGE"}, rows)
		},
	}

	var watchOpts v1.ContainerListOptions
	watchCmd := &cobra.Command{
		Use:   "watch",
		Short: "Stream container changes until interrupted",
		Args:  cobra.NoArgs,
		RunE: func(c *cobra.Command, _ []string) error {
			cl, err := e.connect()
			if err != nil {
				return err
			}
			defer cl.Close()
			w, err := cl.Containers().Watch(c.Context(), watchOpts)
			if err != nil {
				return err
			}
			defer w.Stop()
			return streamEvents(c.Context(), e, w.ResultChan(), w.Err, func(ct v1.Container) string {
				return fmt.Sprintf("%s %s %s %s", ct.ID, ct.NodeID, ct.Image, ct.Status)
			})
		},
	}
	watchCmd.Flags().StringVar(&watchOpts.NodeID, "node", "", "only containers on this node ID")

	cmd.AddCommand(list, get, create, del, events, watchCmd)
	return cmd
}

func (e *env) containerByID(use, short string, fn func(*cobra.Command, string) (*v1.Container, error)) *cobra.Command {
	return &cobra.Command{
		Use:   use + " ID",
		Short: short,
		Args:  cobra.ExactArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			ct, err := fn(c, args[0])
			if err != nil {
				return err
			}
			return e.print(ct, containerHeader, [][]string{containerRow(*ct)})
		},
	}
}
