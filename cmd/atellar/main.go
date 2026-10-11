// atellar is the single Atellar binary: cluster CLI, API server and node
// agent. Run `atellar --help` for the command list.
package main

import "github.com/hasirciogluhq/atellar/cmd/atellar/internal/cli"

func main() { cli.Execute() }
