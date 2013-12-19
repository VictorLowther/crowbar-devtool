package commands

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	buildutils "github.com/VictorLowther/crowbar-devtool/build"
	dev "github.com/VictorLowther/crowbar-devtool/devtool"
	"github.com/VictorLowther/go-git/git"
	c "github.com/gonuts/commander"
	"github.com/gonuts/flag"
)

var baseCommand *c.Command

func addCommand(parent *c.Command, cmd *c.Command) {
	if parent == nil {
		parent = baseCommand
	}
	parent.Subcommands = append(parent.Subcommands, cmd)
	return
}

func addSubCommand(parent *c.Command, subcmd *c.Command) *c.Command {
	if parent == nil {
		parent = baseCommand
	}
	subcmd.Parent = parent
	subcmd.Subcommands = make([]*c.Command, 0, 2)
	parent.Subcommands = append(parent.Subcommands, subcmd)
	return subcmd
}

func showCrowbar(cmd *c.Command, args []string) error {
	dev.MustFindCrowbar()
	log.Printf("Crowbar is located at: %s\n", dev.Repo.Path())
	return nil
}

func fetch(cmd *c.Command, args []string) error {
	dev.MustFindCrowbar()
	ok, _ := dev.Fetch(nil)
	if !ok {
		return fmt.Errorf("crowbar: could not fetch all updates")
	}
	log.Printf("All updates fetched.\n")
	return nil
}

func sync(cmd *c.Command, args []string) error {
	dev.MustFindCrowbar()
	ok, _ := dev.IsClean()
	if !ok {
		log.Printf("Cannot rebase local changes, Crowbar is not clean.\n")
		return isClean(cmd, args)
	}
	ok, res := dev.Rebase()
	if ok {
		log.Println("All local changes rebased against upstream.")
		return nil
	}
	for _, tok := range res {
		log.Printf("%v: %v %v\n", tok.Name, tok.OK, tok.Results)
	}
	log.Println("Errors rebasing local changes.  All changes unwound.")
	return fmt.Errorf("crowbar: error rebasing local changes")
}

func isClean(cmd *c.Command, args []string) error {
	dev.MustFindCrowbar()
	ok, items := dev.IsClean()
	if ok {
		log.Println("All Crowbar repositories are clean.")
		return nil
	}
	for _, item := range items {
		if !item.OK {
			log.Printf("%s is not clean:\n", item.Name)
			for _, line := range item.Results.(git.StatLines) {
				log.Printf("\t%s\n", line.Print())
			}
		}
	}

	return fmt.Errorf("crowbar: repositories not clean")
}

func currentRelease(cmd *c.Command, args []string) error {
	dev.MustFindCrowbar()
	fmt.Println(dev.CurrentRelease().Name())
	return nil
}

func showBuild(cmd *c.Command, args []string) error {
	dev.MustFindCrowbar()
	fmt.Println(dev.CurrentBuild().FullName())
	return nil
}

func releases(cmd *c.Command, args []string) error {
	dev.MustFindCrowbar()
	res := make([]string, 0, 20)
	for release := range dev.Releases() {
		res = append(res, release)
	}
	sort.Strings(res)
	for _, release := range res {
		fmt.Println(release)
	}
	return nil
}

func builds(cmd *c.Command, args []string) error {
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
	return nil
}

func localChanges(cmd *c.Command, args []string) error {
	dev.MustFindCrowbar()
	switch len(args) {
	case 0:
		dev.LocalChanges(dev.CurrentRelease())
	case 1:
		dev.LocalChanges(dev.GetRelease(args[0]))
	default:
		return fmt.Errorf("%s takes 0 or 1 release name!\n", cmd.Name())
	}
	return nil
}

func remoteChanges(cmd *c.Command, args []string) error {
	dev.MustFindCrowbar()
	switch len(args) {
	case 0:
		dev.RemoteChanges(dev.CurrentRelease())
	case 1:
		dev.RemoteChanges(dev.GetRelease(args[0]))
	default:
		return fmt.Errorf("%s takes 0 or 1 release name!\n", cmd.Name())
	}
	return nil
}

func crossReleaseChanges(cmd *c.Command, args []string) error {
	dev.MustFindCrowbar()
	if len(args) != 2 {
		return fmt.Errorf("%s takes exactly 2 release names!")
	}
	releases := new([2]dev.Release)
	// Translate command line parameters.
	// releases[0] will be the release with changes, and
	// releases[1] will be the base release.
	for i, name := range args {
		switch name {
		case "current":
			releases[i] = dev.CurrentRelease()
		case "parent":
			if i == 0 {
				return fmt.Errorf("parent can only be the second arg to %s\n", cmd.Name())
			}
			releases[1] = releases[0].Parent()
			if releases[1] == nil {
				return fmt.Errorf("%s does not have a parent release.\n", releases[0].Name())
			}
		default:
			releases[i] = dev.GetRelease(name)
		}
	}
	dev.CrossReleaseChanges(releases[0], releases[1])
	return nil
}

func barclampsInBuild(cmd *c.Command, args []string) error {
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
			return fmt.Errorf("No such build %s", args[0])
		}
	}
	for name := range dev.BarclampsInBuild(build) {
		res = append(res, name)
	}
	sort.Strings(res)
	for _, name := range res {
		fmt.Println(name)
	}
	return nil
}

func cloneBarclamps(cmd *c.Command, args []string) error {
	dev.MustFindCrowbar()
	dev.CloneBarclamps()
	return nil
}

func switchBuild(cmd *c.Command, args []string) error {
	dev.MustFindCrowbar()
	if ok, _ := dev.IsClean(); !ok {
		return fmt.Errorf("Crowbar is not clean, cannot switch builds.")
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
		return fmt.Errorf("switch takes 0 or 1 argument.")
	}
	if !found {
		return fmt.Errorf("%s is not anything we can switch to!")
	}
	ok, tokens := dev.Switch(target)
	for _, tok := range tokens {
		if tok.Results != nil {
			log.Printf("%s: %v\n", tok.Name, tok.Results)
		}
	}
	if ok {
		log.Printf("Switched to %s\n", target.FullName())
		return nil
	}
	log.Printf("Failed to switch to %s!\n", target.FullName())
	ok, _ = dev.Switch(current)
	return fmt.Errorf("Failed to switch to %s", target.FullName())
}

func update(cmd *c.Command, args []string) error {
	err := fetch(cmd, args)
	if err != nil {
		return err
	}
	return sync(cmd, args)
}

func addRemote(cmd *c.Command, args []string) error {
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
			return fmt.Errorf("Last argument must be a number, but you passed %v\n", args[2])
		}
	default:
		return fmt.Errorf("Adding a remote takes at least 1 and most 3 parameters!")
	}
	dev.ValidateRemote(remote)
	dev.MustFindCrowbar()
	if dev.Remotes[remote.Name] != nil {
		return fmt.Errorf("%s is already a Crowbar remote.", remote.Name)
	}
	dev.AddRemote(remote)
	return nil
}

func zapRemote(cmd *c.Command, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("remote rm only accepts one argument!\n")
	}
	dev.MustFindCrowbar()
	remote, found := dev.Remotes[args[0]]
	if !found {
		return fmt.Errorf("%s is not a remote!\n", args[0])
	}
	dev.ZapRemote(remote)
	return nil
}

func zapBuild(cmd *c.Command, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("remove-build only accepts one argument!\n")
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
		return fmt.Errorf("%s is not a build, cannot delete it!", buildName)
	}
	if strings.HasSuffix(buildName, "/master") {
		return fmt.Errorf("Cannot delete the master build in a release!")
	}
	if err := build.Zap(); err != nil {
		return err
	}
	log.Printf("Build %s deleted.\n", buildName)
	return nil
}

func removeRelease(cmd *c.Command, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("remove-release only accepts one argument!")
	}
	dev.MustFindCrowbar()
	releaseName := args[0]
	releases := dev.Releases()
	release, found := releases[releaseName]
	if !found {
		return fmt.Errorf("%s is not a release!\n", releaseName)
	}
	if releaseName == "development" {
		return fmt.Errorf("Cannot delete the development release.")
	}
	if err := dev.RemoveRelease(release); err != nil {
		return err
	}
	log.Printf("Release %s deleted.\n", releaseName)
	return nil
}

func splitRelease(cmd *c.Command, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("split-release only accepts one argument!")
	}
	dev.MustFindCrowbar()
	current := dev.CurrentRelease()
	if _, err := dev.SplitRelease(current, args[0]); err != nil {
		log.Println(err)
		return fmt.Errorf("Could not split new release %s from %s", args[0], current.Name())
	}
	return nil
}

func showRelease(cmd *c.Command, args []string) error {
	dev.MustFindCrowbar()
	if len(args) == 0 {
		dev.ShowRelease(dev.CurrentRelease())
	} else {
		for _, rel := range args {
			dev.ShowRelease(dev.GetRelease(rel))
		}
	}
	return nil
}

func renameRemote(cmd *c.Command, args []string) error {
	if len(args) != 2 {
		return fmt.Errorf("remote rename takes exactly 2 arguments.\n")
	}
	dev.MustFindCrowbar()
	remote, found := dev.Remotes[args[0]]
	if !found {
		return fmt.Errorf("%s is not a Crowbar remote.", args[0])
	}
	if _, found = dev.Remotes[args[1]]; found {
		return fmt.Errorf("%s is already a remote, cannot rename %s to it\n", args[1], args[0])
	}
	dev.RenameRemote(remote, args[1])
	return nil
}

func updateTracking(cmd *c.Command, args []string) error {
	dev.MustFindCrowbar()
	ok, res := dev.UpdateTrackingBranches()
	if ok {
		return nil
	}
	log.Printf("Failed to update tracking branches in: ")
	for _, result := range res {
		if !result.OK {
			log.Printf("\t%s\n", result.Name)
		}
	}
	return fmt.Errorf("Failed to update tracking branches")
}

func listRemotes(cmd *c.Command, args []string) error {
	dev.MustFindCrowbar()
	for _, remote := range dev.SortedRemotes() {
		fmt.Printf("%s: urlbase=%s, priority=%d\n", remote.Name, remote.Urlbase, remote.Priority)
	}
	return nil
}

func showRemote(cmd *c.Command, args []string) error {
	dev.MustFindCrowbar()
	if len(args) != 1 {
		return fmt.Errorf("Need exactly 1 argument.")
	}
	remote, found := dev.Remotes[args[0]]
	if !found {
		return fmt.Errorf("%s is not a remote!\n", args[0])
	}
	fmt.Printf("Remote %s:\n\tUrlbase: %s\n\tPriority: %d\n", remote.Name, remote.Urlbase, remote.Priority)
	return nil
}

func syncRemotes(cmd *c.Command, args []string) error {
	dev.MustFindCrowbar()
	dev.SyncRemotes()
	return nil
}

func setRemoteURLBase(cmd *c.Command, args []string) error {
	dev.MustFindCrowbar()
	if len(args) != 2 {
		return fmt.Errorf("Need exactly 2 arguments")
	}
	remote, found := dev.Remotes[args[0]]
	if !found {
		return fmt.Errorf("%s is not a remote!\n", args[0])
	}
	dev.SetRemoteURLBase(remote, args[1])
	return nil
}

func sanityCheckBuild(cmd *c.Command, args []string) error {
	dev.MustFindCrowbar()
	paths := make([]string, 0, 0)
	for _, bc := range dev.BarclampsInBuild(dev.CurrentBuild()) {
		paths = append(paths, filepath.Join(bc.Repo.Path(), "crowbar.yml"))
	}
	buildutils.SanityCheckMetadata(paths)
	return nil
}

func init() {
	baseCommand = &c.Command{
		UsageLine: "dev",
		Flag:      *flag.NewFlagSet("dev", flag.ExitOnError),
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
	addCommand(nil, &c.Command{
		Run:       cloneBarclamps,
		UsageLine: "clone-barclamps",
		Short:     "Attempts to clone any missing barclamps.",
	})
	addCommand(nil, &c.Command{
		Run:       sanityCheckBuild,
		UsageLine: "build-sane",
		Short:     "Sanity-check the build metadata for the current build.",
	})

	// Release Handling commands
	release := addSubCommand(nil, &c.Command{
		UsageLine: "release",
		Short:     "Subcommands dealing with releases",
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
		Run:       crossReleaseChanges,
		UsageLine: "changes [target] [base]",
		Short:     "Show commits that are in the target release that are not in the base release.",
	})

	// Remote Management commands.
	remote := addSubCommand(nil, &c.Command{
		UsageLine: "remote",
		Short:     "Subcommands dealing with remote manipulation",
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
	err := baseCommand.Dispatch(os.Args[1:])
	if err != nil {
		fmt.Printf("**err**: %v\n", err)
		os.Exit(1)
	}
	return
}
