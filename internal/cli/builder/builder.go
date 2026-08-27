// Copyright (c) 2026 vlhltf. All rights reserved.
// Use of this source code is governed by a proprietary license.

package builder

import (
	"context"
	"errors"
	"time"

	"github.com/spf13/cobra"
	"google.golang.org/grpc"

	"github.com/lemon4ksan/g-man-cli/internal/cli/output"
	pb "github.com/lemon4ksan/g-man-cli/proto/daemon"
)

// IPCProviderFunc resolves an active IPC gRPC client connection.
type IPCProviderFunc func() (*grpc.ClientConn, pb.DaemonServiceClient, error)

// GlobalIPCProvider is injected by the root command to resolve IPC connections.
var GlobalIPCProvider IPCProviderFunc

// Context encapsulates execution dependencies for a CLI command invocation.
type Context struct {
	Ctx      context.Context
	Cmd      *cobra.Command
	Args     []string
	Conn     *grpc.ClientConn
	Client   pb.DaemonServiceClient
	Renderer *output.Renderer
}

// Render formats output through the active Renderer.
func (c *Context) Render(data any, humanPrinter func() error) error {
	return c.Renderer.Render(data, humanPrinter)
}

// ActionFunc represents a declarative command execution handler.
type ActionFunc func(c *Context) error

// CommandBuilder simplifies declarative construction of Cobra CLI commands.
type CommandBuilder struct {
	cmd     *cobra.Command
	timeout time.Duration
	needIPC bool
	action  ActionFunc
}

// New creates a new CommandBuilder for the given command name.
func New(name string) *CommandBuilder {
	return &CommandBuilder{
		cmd: &cobra.Command{
			Use: name,
		},
		timeout: 5 * time.Minute,
		needIPC: true,
	}
}

// Short sets the one-line description of the command.
func (b *CommandBuilder) Short(desc string) *CommandBuilder {
	b.cmd.Short = desc
	return b
}

// Long sets the detailed multi-line description of the command.
func (b *CommandBuilder) Long(desc string) *CommandBuilder {
	b.cmd.Long = desc
	return b
}

// Aliases registers alternative command invocation names.
func (b *CommandBuilder) Aliases(aliases ...string) *CommandBuilder {
	b.cmd.Aliases = aliases
	return b
}

// Args configures positional arguments validation.
func (b *CommandBuilder) Args(args cobra.PositionalArgs) *CommandBuilder {
	b.cmd.Args = args
	return b
}

// WithTimeout configures the maximum context execution timeout.
func (b *CommandBuilder) WithTimeout(d time.Duration) *CommandBuilder {
	b.timeout = d
	return b
}

// NoIPC marks that this command does not require an active daemon IPC connection.
func (b *CommandBuilder) NoIPC() *CommandBuilder {
	b.needIPC = false
	return b
}

// AddCommand registers subcommands under this command.
func (b *CommandBuilder) AddCommand(cmds ...*cobra.Command) *CommandBuilder {
	b.cmd.AddCommand(cmds...)
	return b
}

// Run attaches the execution action and returns the built *cobra.Command.
func (b *CommandBuilder) Run(fn ActionFunc) *cobra.Command {
	b.action = fn

	b.cmd.RunE = func(cmd *cobra.Command, args []string) error {
		isJSON, _ := cmd.Flags().GetBool("json")
		noColor, _ := cmd.Flags().GetBool("no-color")

		renderer := output.NewRenderer(isJSON, noColor)

		var (
			ctx    context.Context
			cancel context.CancelFunc
		)

		if b.timeout > 0 {
			ctx, cancel = context.WithTimeout(cmd.Context(), b.timeout)
		} else {
			ctx, cancel = context.WithCancel(cmd.Context())
		}

		defer cancel()

		var (
			conn   *grpc.ClientConn
			client pb.DaemonServiceClient
		)

		if b.needIPC {
			if GlobalIPCProvider == nil {
				return errors.New("internal error: IPC provider not registered")
			}

			var err error

			conn, client, err = GlobalIPCProvider()
			if err != nil {
				renderer.Errorf("Error connecting to daemon: %v", err)
				renderer.Warning("Is the daemon 'g-mand' running?")
				return err
			}
			defer conn.Close()
		}

		c := &Context{
			Ctx:      ctx,
			Cmd:      cmd,
			Args:     args,
			Conn:     conn,
			Client:   client,
			Renderer: renderer,
		}

		return b.action(c)
	}

	return b.cmd
}

// Build finalizes and returns the configured *cobra.Command.
func (b *CommandBuilder) Build() *cobra.Command {
	return b.cmd
}
