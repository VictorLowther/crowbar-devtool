package help

import (
	"github.com/gonuts/commander"
	"github.com/gonuts/flag"
)

var commands *commander.Commander

func init() {
	commands = &commander.Commander{
		Name: "dev",
		Flag: flag.NewFlagSet("dev", flag.ExitOnError),
	}
}

func AddCommand(cmd *commander.Command) {
	commands.Command = append(commands.Command,cmd)
	return
}

func AddSubCommand(subcmd *commander.Commander) {
	commands.Commanders = append(commands.Commanders,subcmd)
}