package cli

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	v1 "github.com/hasirciogluhq/atellar/pkg/apis/core/v1"
)

func tokenRow(t v1.JoinToken) []string {
	used, expires := "-", "never"
	if t.UsedBy != nil {
		used = *t.UsedBy
	}
	if t.ExpiresAt != nil {
		expires = t.ExpiresAt.Format(time.RFC3339)
	}
	return []string{t.ID, fmt.Sprint(t.SingleUse), used, expires}
}

var tokenHeader = []string{"ID", "SINGLE_USE", "USED_BY", "EXPIRES"}

func (e *env) joinTokensCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "join-tokens", Aliases: []string{"join-token", "tokens"}, Short: "Manage node join tokens"}

	list := &cobra.Command{
		Use:   "list",
		Short: "List join tokens (secrets are never shown again)",
		Args:  cobra.NoArgs,
		RunE: func(c *cobra.Command, _ []string) error {
			cl, err := e.connect()
			if err != nil {
				return err
			}
			defer cl.Close()
			items, err := cl.JoinTokens().List(c.Context())
			if err != nil {
				return err
			}
			rows := make([][]string, 0, len(items))
			for _, t := range items {
				rows = append(rows, tokenRow(t))
			}
			return e.print(items, tokenHeader, rows)
		},
	}

	var (
		ttl      time.Duration
		multiUse bool
	)
	create := &cobra.Command{
		Use:   "create",
		Short: "Create a join token and print its secret once",
		Args:  cobra.NoArgs,
		RunE: func(c *cobra.Command, _ []string) error {
			req := v1.CreateJoinTokenRequest{}
			if ttl > 0 {
				exp := time.Now().Add(ttl)
				req.ExpiresAt = &exp
			}
			if multiUse {
				single := false
				req.SingleUse = &single
			}
			cl, err := e.connect()
			if err != nil {
				return err
			}
			defer cl.Close()
			t, err := cl.JoinTokens().Create(c.Context(), req)
			if err != nil {
				return err
			}
			if e.output == "json" {
				return e.print(t, nil, nil)
			}
			fmt.Fprintf(e.out, "token: %s\n\nJoin a node with:\n  sudo atellar node join --join-token %s --name <NAME> --private-ip <IP> --control-plane-address <API HOST>\n", t.Token, t.Token)
			return nil
		},
	}
	create.Flags().DurationVar(&ttl, "ttl", 24*time.Hour, "lifetime; 0 never expires")
	create.Flags().BoolVar(&multiUse, "multi-use", false, "allow more than one node to join with this token")

	cmd.AddCommand(list, create)
	return cmd
}
