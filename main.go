package main

import (
	"fmt"
	"github.com/VictorLowther/go-git/git"
	"github.com/gonuts/commander"
	"github.com/gonuts/flag"
	"os"
	"path/filepath"
)

var commands *commander.Commander

func findCrowbar(path string) (repo *git.Repo) {
	var err error
	if path == "" {
		path, err = os.Getwd()
		if err != nil {
			panic("Cannot find current directory!")
		}
	}
	path, err = filepath.Abs(path)
	if err != nil {
		panic("Cannot find absolute path for current directory!")
	}
	repo, err = git.Open(path)
	if err != nil {
		panic("Cannot find git repository containing Crowbar!")
	}
	path = repo.Path()
	parent := filepath.Dir(path)
	// If this is a raw repo, recurse and keep looking.
	if repo.IsRaw() {
		return findCrowbar(parent)
	}
	// See if we have something that looks like a crowbar repo here.
	stat, err := os.Stat(filepath.Join(path, "barclamps"))
	if err != nil || !stat.IsDir() {
		return findCrowbar(parent)
	}
	// We do.  Yays.
	return repo
}

func showCrowbar(cmd *commander.Command, args []string) {
	r := findCrowbar("")
	fmt.Printf("Crowbar is located at: %s\n", r.Path())
}

func show_cmd() *commander.Command {
	cmd := &commander.Command{
		Run:       showCrowbar,
		UsageLine: "show",
		Short:     "Shows the location of the top level Crowbar repo",
	}
	return cmd
}

func init() {
	commands = &commander.Commander{
		Name: "dev",
		Commands: []*commander.Command{
			show_cmd(),
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
