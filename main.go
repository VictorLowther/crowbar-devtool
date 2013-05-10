package main

import (
	"fmt"
	"github.com/VictorLowther/crowbar-devtool/devtool"
	"github.com/gonuts/commander"
	"github.com/gonuts/flag"
	"os"
)

var commands *commander.Commander

func show_cmd() *commander.Command {
	cmd := &commander.Command{
		Run:       devtool.ShowCrowbar,
		UsageLine: "show",
		Short:     "Shows the location of the top level Crowbar repo",
	}
	return cmd
}

func fetch_cmd() *commander.Command {
	return &commander.Command{
		Run: devtool.Fetch,
		UsageLine: "fetch",
		Short: "Fetches updates from all remotes",
	}
}

func init() {
	commands = &commander.Commander{
		Name: "dev",
		Commands: []*commander.Command{
			show_cmd(),
			fetch_cmd(),
		},
		Flag: flag.NewFlagSet("dev", flag.ExitOnError),
	}
}

func main() {
	err := commands.Flag.Parse(os.Args[1:])
	if err != nil {
		fmt.Printf("**err**: %v\n", err)
		os.Exit(1)
	}
	args := commands.Flag.Args()
	err = commands.Run(args)
	if err != nil {
		fmt.Printf("**err**: %v\n", err)
		os.Exit(1)
	}
	return
}
