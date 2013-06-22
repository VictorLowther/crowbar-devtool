package commands

import (
	"fmt"
	dev "github.com/VictorLowther/crowbar-devtool/devtool"
	"github.com/VictorLowther/go-git/git"
	c "github.com/gonuts/commander"
	"github.com/gonuts/flag"
	"log"
	"os"
	"sort"
	"strconv"
	"strings"
)

var baseCommand *c.Commander

func addCommand(parent *c.Commander, cmd *c.Command) {
	if parent == nil {
		parent = baseCommand
	}
	parent.Commands = append(parent.Commands, cmd)
	return
}

func addSubCommand(parent *c.Commander, subcmd *c.Commander) *c.Commander {
	if parent == nil {
		parent = baseCommand
	}
	subcmd.Parent = parent
	subcmd.Commands = make([]*c.Command, 0, 2)
	subcmd.Commanders = make([]*c.Commander, 0, 1)
	parent.Commanders = append(parent.Commanders, subcmd)
	return subcmd
}

func showCrowbar(cmd *c.Command, args []string) {
	dev.MustFindCrowbar()
	log.Printf("Crowbar is located at: %s\n", dev.Repo.Path())
}

func fetch(cmd *c.Command, args []string) {
	dev.MustFindCrowbar()
	ok, _ := dev.Fetch(nil)
	if !ok {
		os.Exit(1)
	}
	log.Printf("All updates fetched.\n")
}

func sync(cmd *c.Command, args []string) {
	dev.MustFindCrowbar()
	ok, _ := dev.IsClean()
	if !ok {
		log.Printf("Cannot rebase local changes, Crowbar is not clean.\n")
		isClean(cmd, args)
	}
	ok, res := dev.Rebase()
	if ok {
		log.Println("All local changes rebased against upstream.")
		os.Exit(0)
	}
	for _, tok := range res {
		log.Printf("%v: %v %v\n", tok.Name, tok.OK, tok.Results)
	}
	log.Println("Errors rebasing local changes.  All changes unwound.")
	os.Exit(1)
}

func isClean(cmd *c.Command, args []string) {
	dev.MustFindCrowbar()
	ok, items := dev.IsClean()
	if ok {
		log.Println("All Crowbar repositories are clean.")
		os.Exit(0)
	}
	for _, item := range items {
		if !item.OK {
			log.Printf("%s is not clean:\n", item.Name)
			for _, line := range item.Results.(git.StatLines) {
				log.Printf("\t%s\n", line.Print())
			}
		}
	}
	os.Exit(1)
	return
}

func currentRelease(cmd *c.Command, args []string) {
	dev.MustFindCrowbar()
	fmt.Println(dev.CurrentRelease().Name())
}

func showBuild(cmd *c.Command, args []string) {
	dev.MustFindCrowbar()
	fmt.Println(dev.CurrentBuild().FullName())
}

func releases(cmd *c.Command, args []string) {
	dev.MustFindCrowbar()
	res := make([]string, 0, 20)
	for release := range dev.Releases() {
		res = append(res, release)
	}
	sort.Strings(res)
	for _, release := range res {
		fmt.Println(release)
	}
}

func builds(cmd *c.Command, args []string) {
	dev.MustFindCrowbar()
	res := make([]string, 0, 20)
	if len(args) == 0 {
		for build := range dev.CurrentRelease().Builds() {
			res = append(res, dev.CurrentRelease().Name()+"/"+build)
		}
	} else {
		for _, release := range args {
			for build := range dev.GetRelease(release).Builds() {
				res = append(res, release+"/"+build)
			}
		}
	}
	sort.Strings(res)
	for _, build := range res {
		fmt.Println(build)
	}
}

func localChanges(cmd *c.Command, args []string) {
	dev.MustFindCrowbar()
	switch len(args) {
	case 0: dev.LocalChanges(dev.CurrentRelease())
	case 1: dev.LocalChanges(dev.GetRelease(args[0]))
	default: log.Fatalf("%s takes 0 or 1 release name!\n",cmd.Name())
	}
}

func remoteChanges(cmd *c.Command, args []string) {
	dev.MustFindCrowbar()
	switch len(args) {
	case 0: dev.RemoteChanges(dev.CurrentRelease())
	case 1: dev.RemoteChanges(dev.GetRelease(args[0]))
	default: log.Fatalf("%s takes 0 or 1 release name!\n",cmd.Name())
	}
}

func crossReleaseChanges (cmd *c.Command, args []string) {
	dev.MustFindCrowbar()
	if len(args) != 2 {
		log.Fatalf("%s takes exactly 2 release names!")
	}
	releases := new([2]dev.Release)
	// Translate command line parameters.
	// releases[0] will be the release with changes, and
	// releases[1] will be the base release.
	for i,name := range args {
		switch name {
		case "current": releases[i] = dev.CurrentRelease()
		case "parent":
			if i == 0 {
				log.Fatalf("parent can only be the second arg to %s\n",cmd.Name())
			}
			releases[1] = releases[0].Parent()
			if releases[1] == nil {
				log.Fatalf("%s does not have a parent release.\n",releases[0].Name())
			}
		default: releases[i] = dev.GetRelease(name)
		}
	}
	dev.CrossReleaseChanges(releases[0],releases[1])
}

func barclampsInBuild(cmd *c.Command, args []string) {
	dev.MustFindCrowbar()
	res := make([]string, 0, 20)
	var build dev.Build
	var found bool
	if len(args) == 0 {
		build = dev.CurrentBuild()
	} else if len(args) == 1 {
		builds := dev.Builds()
		build, found = builds[args[0]]
		if !found {
			log.Fatalln("No such build %s", args[0])
		}
	}
	for name := range dev.BarclampsInBuild(build) {
		res = append(res, name)
	}
	sort.Strings(res)
	for _, name := range res {
		fmt.Println(name)
	}
}

func switchBuild(cmd *c.Command, args []string) {
	dev.MustFindCrowbar()
	if ok, _ := dev.IsClean(); !ok {
		log.Fatalln("Crowbar is not clean, cannot switch builds.")
	}
	rels := dev.Releases()
	current := dev.CurrentBuild()
	var target dev.Build
	found := false
	switch len(args) {
	case 0:
		target, found = current, true
	case 1:
		// Were we passed a known release?
		rel, foundRel := rels[args[0]]
		if foundRel {
			for _, build := range []string{current.Name(), "master"} {
				target, found = rel.Builds()[build]
				if found {
					break
				}
			}
		} else {
			target, found = dev.Builds()[args[0]]
		}
	default:
		log.Fatalf("switch takes 0 or 1 argument.")
	}
	if !found {
		log.Fatalf("%s is not anything we can switch to!")
	}
	ok, tokens := dev.Switch(target)
	for _, tok := range tokens {
		if tok.Results != nil {
			log.Printf("%s: %v\n", tok.Name, tok.Results)
		}
	}
	if ok {
		log.Printf("Switched to %s\n", target.FullName())
		os.Exit(0)
	}
	log.Printf("Failed to switch to %s!\n", target.FullName())
	ok, _ = dev.Switch(current)
	os.Exit(1)
}

func update(cmd *c.Command, args []string) {
	fetch(cmd, args)
	sync(cmd, args)
}

func addRemote(cmd *c.Command, args []string) {
	remote := &dev.Remote{Priority: 50}
	switch len(args) {
	case 1:
		remote.Urlbase = args[0]
	case 2:
		pri, err := strconv.Atoi(args[1])
		if err == nil {
			remote.Priority = pri
			remote.Urlbase = args[0]
		} else {
			remote.Name, remote.Urlbase = args[0], args[1]
		}
	case 3:
		remote.Name, remote.Urlbase = args[0], args[1]
		pri, err := strconv.Atoi(args[2])
		if err == nil {
			remote.Priority = pri
		} else {
			log.Fatalf("Last argument must be a number, but you passed %v\n", args[2])
		}
	default:
		log.Fatalf("Adding a remote takes at least 1 and most 3 parameters!")
	}
	dev.ValidateRemote(remote)
	dev.MustFindCrowbar()
	if dev.Remotes[remote.Name] != nil {
		log.Fatalf("%s is already a Crowbar remote.", remote.Name)
	}
	dev.AddRemote(remote)
	os.Exit(0)
}

func zapRemote(cmd *c.Command, args []string) {
	if len(args) != 1 {
		log.Fatalf("remote rm only accepts one argument!\n")
	}
	dev.MustFindCrowbar()
	remote, found := dev.Remotes[args[0]]
	if !found {
		log.Fatalf("%s is not a remote!\n", args[0])
	}
	dev.ZapRemote(remote)
}

func zapBuild(cmd *c.Command, args []string) {
	if len(args) != 1 {
		log.Fatalf("remove-build only accepts one argument!\n")
	}
	buildName := args[0]
	dev.MustFindCrowbar()
	if !strings.Contains(buildName, "/") {
		// We were passed what appears to be a raw build name.
		// Turn it into a real build by prepending the release name.
		buildName = dev.CurrentRelease().Name() + "/" + buildName
	}
	builds := dev.Builds()
	build, found := builds[buildName]
	if !found {
		log.Fatalf("%s is not a build, cannot delete it!", buildName)
	}
	if strings.HasSuffix(buildName, "/master") {
		log.Fatalf("Cannot delete the master build in a release!")
	}
	if err := build.Zap(); err != nil {
		log.Fatal(err)
	}
	log.Printf("Build %s deleted.\n", buildName)
}

func removeRelease(cmd *c.Command, args []string) {
	if len(args) != 1 {
		log.Fatalf("remove-release only accepts one argument!")
	}
	dev.MustFindCrowbar()
	releaseName := args[0]
	releases := dev.Releases()
	release, found := releases[releaseName]
	if !found {
		log.Fatalf("%s is not a release!\n", releaseName)
	}
	if releaseName == "development" {
		log.Fatal("Cannot delete the development release.")
	}
	if err := dev.RemoveRelease(release); err != nil {
		log.Fatal(err)
	}
	log.Printf("Release %s deleted.\n", releaseName)
}

func splitRelease(cmd *c.Command, args []string) {
	if len(args) != 1 {
		log.Fatalf("split-release only accepts one argument!")
	}
	dev.MustFindCrowbar()
	current := dev.CurrentRelease()
	if _, err := dev.SplitRelease(current, args[0]); err != nil {
		log.Println(err)
		log.Fatalf("Could not split new release %s from %s", args[0], current.Name())
	}
}

func showRelease(cmd *c.Command, args []string) {
	dev.MustFindCrowbar()
	if len(args) == 0 {
		dev.ShowRelease(dev.CurrentRelease())
	} else {
		for _, rel := range args {
			dev.ShowRelease(dev.GetRelease(rel))
		}
	}
}

func renameRemote(cmd *c.Command, args []string) {
	if len(args) != 2 {
		log.Fatalf("remote rename takes exactly 2 arguments.\n")
	}
	dev.MustFindCrowbar()
	remote, found := dev.Remotes[args[0]]
	if !found {
		log.Fatalf("%s is not a Crowbar remote.", args[0])
	}
	if _, found = dev.Remotes[args[1]]; found {
		log.Fatalf("%s is already a remote, cannot rename %s to it\n", args[1], args[0])
	}
	dev.RenameRemote(remote, args[1])
}

func updateTracking(cmd *c.Command, args []string) {
	dev.MustFindCrowbar()
	ok, res := dev.UpdateTrackingBranches()
	if ok {
		os.Exit(0)
	}
	log.Printf("Failed to update tracking branches in: ")
	for _, result := range res {
		if !result.OK {
			log.Printf("\t%s\n", result.Name)
		}
	}
	os.Exit(1)
}

func listRemotes(cmd *c.Command, args []string) {
	dev.MustFindCrowbar()
	for _, remote := range dev.SortedRemotes() {
		fmt.Printf("%s: urlbase=%s, priority=%d\n", remote.Name, remote.Urlbase, remote.Priority)
	}
	os.Exit(0)
}

func showRemote(cmd *c.Command, args []string) {
	dev.MustFindCrowbar()
	if len(args) != 1 {
		log.Fatal("Need exactly 1 argument.")
	}
	remote, found := dev.Remotes[args[0]]
	if !found {
		log.Fatalf("%s is not a remote!\n", args[0])
	}
	fmt.Printf("Remote %s:\n\tUrlbase: %s\n\tPriority: %d\n", remote.Name, remote.Urlbase, remote.Priority)
	os.Exit(0)
}

func syncRemotes(cmd *c.Command, args []string) {
	dev.MustFindCrowbar()
	dev.SyncRemotes()
}

func setRemoteURLBase(cmd *c.Command, args []string) {
	dev.MustFindCrowbar()
	if len(args) != 2 {
		log.Fatal("Need exactly 2 arguments")
	}
	remote, found := dev.Remotes[args[0]]
	if !found {
		log.Fatalf("%s is not a remote!\n", args[0])
	}
	dev.SetRemoteURLBase(remote, args[1])
}

func init() {
	baseCommand = &c.Commander{
		Name: "dev",
		Flag: flag.NewFlagSet("dev", flag.ExitOnError),
	}
	// Core Crowbar commands.
	addCommand(nil, &c.Command{
		Run:       isClean,
		UsageLine: "clean?",
		Short:     "Shows whether Crowbar overall is clean.",
		Long: `Show whether or not all of the repositories that are part of this
Crowbar checkout are clean.  If they are, this command exits with a zero exit
code. If they are not, this command shows what is dirty in each repository,
and exits with an exit code of 1.`,
	})
	addCommand(nil, &c.Command{
		Run:       releases,
		UsageLine: "releases",
		Short:     "Shows the releases available to work on.",
	})
	addCommand(nil, &c.Command{
		Run:       barclampsInBuild,
		UsageLine: "barclamps-in-build [build]",
		Short:     "Shows the releases available to work on.",
	})
	addCommand(nil, &c.Command{
		Run:       builds,
		UsageLine: "builds",
		Short:     "Shows the builds in a release or releases.",
	})
	addCommand(nil, &c.Command{
		Run:       showBuild,
		UsageLine: "branch",
		Short:     "Shows the current branch",
	})
	addCommand(nil, &c.Command{
		Run:       showCrowbar,
		UsageLine: "show",
		Short:     "Shows the location of the top level Crowbar repo",
	})
	addCommand(nil, &c.Command{
		Run:       fetch,
		UsageLine: "fetch",
		Short:     "Fetches updates from all remotes",
	})
	addCommand(nil, &c.Command{
		Run:       sync,
		UsageLine: "sync",
		Short:     "Rebase local changes on their tracked upstream changes.",
	})
	addCommand(nil, &c.Command{
		Run:       switchBuild,
		UsageLine: "switch [build or release]",
		Short:     "Switch to the named release or build",
	})
	addCommand(nil, &c.Command{
		Run:       update,
		UsageLine: "update",
		Short:     "Fetch all changes from upstream and then rebase local changes on top of them.",
	})
	addCommand(nil, &c.Command{
		Run:       zapBuild,
		UsageLine: "remove-build [build]",
		Short:     "Remove a non-master build with no children.",
	})
	addCommand(nil, &c.Command{
		Run:       localChanges,
		UsageLine: "local-changes [release]",
		Short:     "Show local changes that have not been comitted upstream.",
	})
	addCommand(nil, &c.Command{
		Run:       remoteChanges,
		UsageLine: "remote-changes [release]",
		Short:     "Show changes that have been comitted upstream, but that are not present locally.",
	})

	// Release Handling commands
	release := addSubCommand(nil, &c.Commander{
		Name:  "release",
		Short: "Subcommands dealing with releases",
	})
	addCommand(release, &c.Command{
		Run:       removeRelease,
		UsageLine: "remove [release]",
		Short:     "Remove a release.",
	})
	addCommand(release, &c.Command{
		Run:       splitRelease,
		UsageLine: "new [new-name]",
		Short:     "Create a new release from the current release.",
	})
	addCommand(release, &c.Command{
		Run:       currentRelease,
		UsageLine: "current",
		Short:     "Shows the current release",
	})
	addCommand(release, &c.Command{
		Run:       releases,
		UsageLine: "list",
		Short:     "Shows the releases available to work on.",
	})
	addCommand(release, &c.Command{
		Run:       showRelease,
		UsageLine: "show",
		Short:     "Shows details about the current or passed release",
	})
	addCommand(release, &c.Command{
		Run: crossReleaseChanges,
		UsageLine: "changes [target] [base]",
		Short: "Show commits that are in the target release that are not in the base release.",
	})

	// Remote Management commands.
	remote := addSubCommand(nil, &c.Commander{
		Name:  "remote",
		Short: "Subcommands dealing with remote manipulation",
	})
	addCommand(remote, &c.Command{
		Run:       updateTracking,
		UsageLine: "retrack",
		Short:     "Update tracking references for all branches across all releases.",
	})
	addCommand(remote, &c.Command{
		Run:       listRemotes,
		UsageLine: "list",
		Short:     "List the remotes Crowbar is configured to use.",
	})
	addCommand(remote, &c.Command{
		Run:       showRemote,
		UsageLine: "show [remote]",
		Short:     "Show details about a specific remote",
	})
	addCommand(remote, &c.Command{
		Run:       addRemote,
		UsageLine: "add [remote] [URL] [priority]",
		Short:     "Add a new remote",
	})
	addCommand(remote, &c.Command{
		Run:       zapRemote,
		UsageLine: "rm [remote]",
		Short:     "Remove a remote.",
	})
	addCommand(remote, &c.Command{
		Run:       renameRemote,
		UsageLine: "rename [oldname] [newname]",
		Short:     "Rename a remote.",
	})
	addCommand(remote, &c.Command{
		Run:       setRemoteURLBase,
		UsageLine: "set-urlbase [remote] [urlbase]",
		Short:     "Set a new URL for a remote.",
	})
	addCommand(remote, &c.Command{
		Run:       syncRemotes,
		UsageLine: "sync",
		Short:     "Recalculate and synchronize remotes across all repositories.",
	})
	return
}

// Run is the main entry point for actually running a dev command.
func Run() {
	err := baseCommand.Flag.Parse(os.Args[1:])
	if err != nil {
		fmt.Printf("**err**: %v\n", err)
		os.Exit(1)
	}
	args := baseCommand.Flag.Args()
	err = baseCommand.Run(args)
	if err != nil {
		fmt.Printf("**err**: %v\n", err)
		os.Exit(1)
	}
	return
}
