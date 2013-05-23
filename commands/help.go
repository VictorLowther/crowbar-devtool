package commands

import (
	"fmt"
	"github.com/gonuts/commander"
	"github.com/gonuts/flag"
	"os"
)

var base_command *commander.Commander

func init() {
	base_command = &commander.Commander{
		Name: "dev",
		Flag: flag.NewFlagSet("dev", flag.ExitOnError),
	}
}

func AddCommand(parent *commander.Commander, cmd *commander.Command) {
	if parent == nil {
		parent = base_command
	}
	parent.Commands = append(parent.Commands, cmd)
	return
}

func AddSubCommand(parent *commander.Commander, subcmd *commander.Commander) *commander.Commander {
	if parent == nil {
		parent = base_command
	}
	subcmd.Parent = parent
	subcmd.Commands = make([]*commander.Command, 0, 2)
	subcmd.Commanders = make([]*commander.Commander, 0, 1)
	parent.Commanders = append(parent.Commanders, subcmd)
	return subcmd
}

func Run() {
	err := base_command.Flag.Parse(os.Args[1:])
	if err != nil {
		fmt.Printf("**err**: %v\n", err)
		os.Exit(1)
	}
	args := base_command.Flag.Args()
	err = base_command.Run(args)
	if err != nil {
		fmt.Printf("**err**: %v\n", err)
		os.Exit(1)
	}
	return
}
