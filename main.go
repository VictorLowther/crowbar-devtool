package main

import (
	"github.com/VictorLowther/crowbar-devtool/commands"
	"os"
)

// All the actual meat is in the devtool package
func main() {
	commands.Run()
	os.Exit(0)
	return
}
