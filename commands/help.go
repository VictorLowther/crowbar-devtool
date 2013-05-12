package commands

import (
	"github.com/gonuts/commander"
	"github.com/gonuts/flag"
	"os"
	"fmt"
)

var r *commander.Commander

func init() {
	r = &commander.Commander{
		Name: "dev",
		Flag: flag.NewFlagSet("dev", flag.ExitOnError),
	}
}

func AddCommand(cmd *commander.Command) {
	r.Commands = append(r.Commands,cmd)
	return
}

func AddSubCommand(subcmd *commander.Commander) {
	r.Commanders = append(r.Commanders,subcmd)
}

func Run() {
	err := r.Flag.Parse(os.Args[1:])
	if err != nil {
		fmt.Printf("**err**: %v\n", err)
		os.Exit(1)
	}
	args := r.Flag.Args()
	err = r.Run(args)
	if err != nil {
		fmt.Printf("**err**: %v\n", err)
		os.Exit(1)
	}
	return
}