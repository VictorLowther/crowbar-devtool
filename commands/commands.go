package commands

import (
	"fmt"
	"github.com/VictorLowther/crowbar-devtool/devtool"
	"github.com/VictorLowther/go-git/git"
	"github.com/gonuts/commander"
	"github.com/gonuts/flag"
	"log"
	"os"
	"sort"
	"strings"
	"strconv"
)

var base_command *commander.Commander

func addCommand(parent *commander.Commander, cmd *commander.Command) {
	if parent == nil {
		parent = base_command
	}
	parent.Commands = append(parent.Commands, cmd)
	return
}

func addSubCommand(parent *commander.Commander, subcmd *commander.Commander) *commander.Commander {
	if parent == nil {
		parent = base_command
	}
	subcmd.Parent = parent
	subcmd.Commands = make([]*commander.Command, 0, 2)
	subcmd.Commanders = make([]*commander.Commander, 0, 1)
	parent.Commanders = append(parent.Commanders, subcmd)
	return subcmd
}

func ShowCrowbar(cmd *commander.Command, args []string) {
	devtool.MustFindCrowbar()
	log.Printf("Crowbar is located at: %s\n", devtool.Repo.Path())
}

func Fetch(cmd *commander.Command, args []string) {
	devtool.MustFindCrowbar()
	ok, _ := devtool.Fetch(nil)
	if !ok {
		os.Exit(1)
	}
	log.Printf("All updates fetched.\n")
}

func Sync(cmd *commander.Command, args []string) {
	devtool.MustFindCrowbar()
	ok, _ := devtool.IsClean()
	if !ok {
		log.Printf("Cannot rebase local changes, Crowbar is not clean.\n")
		IsClean(cmd, args)
	}
	ok, res := devtool.Rebase()
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

func IsClean(cmd *commander.Command, args []string) {
	devtool.MustFindCrowbar()
	ok, items := devtool.IsClean()
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

func ShowRelease(cmd *commander.Command, args []string) {
	devtool.MustFindCrowbar()
	fmt.Println(devtool.CurrentRelease().Name())
}

func ShowBuild(cmd *commander.Command, args []string) {
	devtool.MustFindCrowbar()
	fmt.Println(devtool.CurrentBuild().FullName())
}

func Releases(cmd *commander.Command, args []string) {
	devtool.MustFindCrowbar()
	res := make([]string, 0, 20)
	for release, _ := range devtool.Releases() {
		res = append(res, release)
	}
	sort.Strings(res)
	for _, release := range res {
		fmt.Println(release)
	}
}

func Builds(cmd *commander.Command, args []string) {
	devtool.MustFindCrowbar()
	res := make([]string, 0, 20)
	if len(args) == 0 {
		for build, _ := range devtool.CurrentRelease().Builds() {
			res = append(res, devtool.CurrentRelease().Name()+"/"+build)
		}
	} else {
		for _, release := range args {
			for build, _ := range devtool.GetRelease(release).Builds() {
				res = append(res, release+"/"+build)
			}
		}
	}
	sort.Strings(res)
	for _, build := range res {
		fmt.Println(build)
	}
}

func BarclampsInBuild(cmd *commander.Command, args []string) {
	devtool.MustFindCrowbar()
	res := make([]string, 0, 20)
	var build devtool.Build
	var found bool
	if len(args) == 0 {
		build = devtool.CurrentBuild()
	} else if len(args) == 1 {
		builds := devtool.Builds()
		build, found = builds[args[0]]
		if !found {
			log.Fatalln("No such build %s", args[0])
		}
	}
	for name, _ := range devtool.BarclampsInBuild(build) {
		res = append(res, name)
	}
	sort.Strings(res)
	for _, name := range res {
		fmt.Println(name)
	}
}

func Switch(cmd *commander.Command, args []string) {
	devtool.MustFindCrowbar()
	if ok, _ := devtool.IsClean(); !ok {
		log.Fatalln("Crowbar is not clean, cannot switch builds.")
	}
	rels := devtool.Releases()
	current := devtool.CurrentBuild()
	var target devtool.Build
	found := false
	switch len(args) {
	case 0:
		target, found = current, true
	case 1:
		// Were we passed a known release?
		rel, found_rel := rels[args[0]]
		if found_rel {
			for _, build := range []string{current.Name(), "master"} {
				target, found = rel.Builds()[build]
				if found {
					break
				}
			}
		} else {
			target, found = devtool.Builds()[args[0]]
		}
	default:
		log.Fatalf("switch takes 0 or 1 argument.")
	}
	if !found {
		log.Fatalf("%s is not anything we can switch to!")
	}
	ok, tokens := devtool.Switch(target)
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
	ok, _ = devtool.Switch(current)
	os.Exit(1)
}

func Update(cmd *commander.Command, args []string) {
	Fetch(cmd, args)
	Sync(cmd, args)
}

func AddRemote(cmd *commander.Command, args []string) {
	remote := &devtool.Remote{Priority: 50}
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
	devtool.ValidateRemote(remote)
	devtool.MustFindCrowbar()
	if devtool.Remotes[remote.Name] != nil {
		log.Fatalf("%s is already a Crowbar remote.", remote.Name)
	}
	devtool.AddRemote(remote)
	os.Exit(0)
}

func ZapRemote(cmd *commander.Command, args []string) {
	if len(args) != 1 {
		log.Fatalf("remote rm only accepts one argument!\n")
	}
	devtool.MustFindCrowbar()
	remote, found := devtool.Remotes[args[0]]
	if !found {
		log.Fatalf("%s is not a remote!\n", args[0])
	}
	devtool.ZapRemote(remote)
}

func ZapBuild(cmd *commander.Command, args []string) {
	if len(args) != 1 {
		log.Fatalf("remove-build only accepts one argument!\n")
	}
	buildName := args[0]
	devtool.MustFindCrowbar()
	if !strings.Contains(buildName,"/") {
		// We were passed what appears to be a raw build name.
		// Turn it into a real build by prepending the release name.
			buildName = devtool.CurrentRelease().Name() + "/" + buildName
	}
	builds := devtool.Builds()
	build,found := builds[buildName]
	if !found {
		log.Fatalf("%s is not a build, cannot delete it!",buildName)
	}
	if strings.HasSuffix(buildName,"/master") {
		log.Fatalf("Cannot delete the master build in a release!")
	}
	if err := build.Zap(); err != nil {
		log.Fatal(err)
	}
	log.Printf("Build %s deleted.\n",buildName)
}

func ZapRelease(cmd *commander.Command, args []string) {
	if len(args) != 1 {
		log.Fatalf("remove-release only accepts one argument!")
	}
	devtool.MustFindCrowbar()
	releaseName := args[0]
	releases := devtool.Releases()
	release,found := releases[releaseName]
	if !found {
		log.Fatalf("%s is not a release!\n",releaseName)
	}
	if releaseName == "development" {
		log.Fatal("Cannot delete the development release.")
	}
	if err := release.Zap(); err != nil {
		log.Fatal(err)
	}
	log.Printf("Release %s deleted.\n",releaseName)
}

func SplitRelease(cmd *commander.Command, args []string) {
	if len(args) != 1 {
		log.Fatalf("split-release only accepts one argument!")
	}
	devtool.MustFindCrowbar()
	current := devtool.CurrentRelease()
	if _,err := devtool.SplitRelease(current,args[0]); err != nil {
		log.Println(err)
		log.Fatalf("Could not split new release %s from %s",args[0],current.Name())
	}
}

func RenameRemote(cmd *commander.Command, args []string) {
	if len(args) != 2 {
		log.Fatalf("remote rename takes exactly 2 arguments.\n")
	}
	devtool.MustFindCrowbar()
	remote, found := devtool.Remotes[args[0]]
	if !found {
		log.Fatalf("%s is not a Crowbar remote.", args[0])
	}
	if _, found = devtool.Remotes[args[1]]; found {
		log.Fatalf("%s is already a remote, cannot rename %s to it\n", args[1], args[0])
	}
	devtool.RenameRemote(remote, args[1])
}

func UpdateTracking(cmd *commander.Command, args []string) {
	devtool.MustFindCrowbar()
	ok, res := devtool.UpdateTrackingBranches()
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

func ListRemotes(cmd *commander.Command, args []string) {
	devtool.MustFindCrowbar()
	for _, remote := range devtool.SortedRemotes() {
		fmt.Printf("%s: urlbase=%s, priority=%d\n", remote.Name, remote.Urlbase, remote.Priority)
	}
	os.Exit(0)
}

func ShowRemote(cmd *commander.Command, args []string) {
	devtool.MustFindCrowbar()
	if len(args) != 1 {
		log.Fatal("Need exactly 1 argument.")
	}
	remote, found := devtool.Remotes[args[0]]
	if !found {
		log.Fatalf("%s is not a remote!\n", args[0])
	}
	fmt.Printf("Remote %s:\n\tUrlbase: %s\n\tPriority: %d\n", remote.Name, remote.Urlbase, remote.Priority)
	os.Exit(0)
}

func SyncRemotes(cmd *commander.Command, args []string) {
	devtool.MustFindCrowbar()
	devtool.SyncRemotes()
}

func SetRemoteURLBase(cmd *commander.Command, args []string) {
	devtool.MustFindCrowbar()
	if len(args) != 2 {
		log.Fatal("Need exactly 2 arguments")
	}
	remote, found := devtool.Remotes[args[0]]
	if !found {
		log.Fatalf("%s is not a remote!\n", args[0])
	}
	devtool.SetRemoteURLBase(remote, args[1])
}

func init() {
	base_command = &commander.Commander{
		Name: "dev",
		Flag: flag.NewFlagSet("dev", flag.ExitOnError),
	}
	// Core Crowbar commands.
	addCommand(nil, &commander.Command{
		Run:       IsClean,
		UsageLine: "clean?",
		Short:     "Shows whether Crowbar overall is clean.",
		Long: `Show whether or not all of the repositories that are part of this
Crowbar checkout are clean.  If they are, this command exits with a zero exit
code. If they are not, this command shows what is dirty in each repository,
and exits with an exit code of 1.`,
	})
	addCommand(nil, &commander.Command{
		Run:       Releases,
		UsageLine: "releases",
		Short:     "Shows the releases available to work on.",
	})
	addCommand(nil, &commander.Command{
		Run:       BarclampsInBuild,
		UsageLine: "barclamps-in-build [build]",
		Short:     "Shows the releases available to work on.",
	})
	addCommand(nil, &commander.Command{
		Run:       Builds,
		UsageLine: "builds",
		Short:     "Shows the builds in a release or releases.",
	})
	addCommand(nil, &commander.Command{
		Run:       ShowRelease,
		UsageLine: "release",
		Short:     "Shows the current release",
	})
	addCommand(nil, &commander.Command{
		Run:       ShowBuild,
		UsageLine: "branch",
		Short:     "Shows the current branch",
	})
	addCommand(nil, &commander.Command{
		Run:       ShowCrowbar,
		UsageLine: "show",
		Short:     "Shows the location of the top level Crowbar repo",
	})
	addCommand(nil, &commander.Command{
		Run:       Fetch,
		UsageLine: "fetch",
		Short:     "Fetches updates from all remotes",
	})
	addCommand(nil, &commander.Command{
		Run:       Sync,
		UsageLine: "sync",
		Short:     "Rebase local changes on their tracked upstream changes.",
	})
	addCommand(nil, &commander.Command{
		Run:       Switch,
		UsageLine: "switch [build or release]",
		Short:     "Switch to the named release or build",
	})
	addCommand(nil, &commander.Command{
		Run:       Update,
		UsageLine: "update",
		Short:     "Fetch all changes from upstream and then rebase local changes on top of them.",
	})
	addCommand(nil, &commander.Command{
		Run: ZapBuild,
		UsageLine: "remove-build [build]",
		Short: "Remove a non-master build with no children.",
	})
	addCommand(nil, &commander.Command{
		Run: ZapRelease,
		UsageLine: "remove-release [release]",
		Short: "Remove a release",
	})
	addCommand(nil, &commander.Command{
		Run: SplitRelease,
		UsageLine: "new-release [new-name]",
		Short: "Create a new release from the current release.",
	})

	// Remote Management commands.
	remote := addSubCommand(nil, &commander.Commander{
		Name:  "remote",
		Short: "Subcommands dealing with remote manipulation",
	})
	addCommand(remote, &commander.Command{
		Run:       UpdateTracking,
		UsageLine: "retrack",
		Short:     "Update tracking references for all branches across all releases.",
	})
	addCommand(remote, &commander.Command{
		Run:       ListRemotes,
		UsageLine: "list",
		Short:     "List the remotes Crowbar is configured to use.",
	})
	addCommand(remote, &commander.Command{
		Run:       ShowRemote,
		UsageLine: "show [remote]",
		Short:     "Show details about a specific remote",
	})
	addCommand(remote, &commander.Command{
		Run:       AddRemote,
		UsageLine: "add [remote] [URL] [priority]",
		Short:     "Add a new remote",
	})
	addCommand(remote, &commander.Command{
		Run:       ZapRemote,
		UsageLine: "rm [remote]",
		Short:     "Remove a remote.",
	})
	addCommand(remote, &commander.Command{
		Run:       RenameRemote,
		UsageLine: "rename [oldname] [newname]",
		Short:     "Rename a remote.",
	})
	addCommand(remote, &commander.Command{
		Run:       SetRemoteURLBase,
		UsageLine: "set-urlbase [remote] [urlbase]",
		Short:     "Set a new URL for a remote.",
	})
	addCommand(remote, &commander.Command{
		Run:       SyncRemotes,
		UsageLine: "sync",
		Short:     "Recalculate and synchronize remotes across all repositories.",
	})
	return
}

// Main entry point for actually running a devtool command.
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
