package main

import (
	"github.com/VictorLowther/crowbar-devtool/devtool"
	"os"
)
// All the actual meat is in the devtool package
func main() {
	devtool.Run()
	os.Exit(0)
	return
}
