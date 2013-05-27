package devtool

import (
	"errors"
	"fmt"
	"github.com/VictorLowther/crowbar-devtool/commands"
	"github.com/VictorLowther/go-git/git"
	"github.com/gonuts/commander"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// Placeholder for a barclamp.
// Barclamps are leaf nodes from the point of view of the
// release branching stricture.
type Barclamp struct {
	// All barclamps have to have a name.
	Name string
	// This is the branch in git that the barclamp should
	// be checked out to for a build.
	Branch string
	// The git repo that holds the actual code for the barclamp.
	Repo *git.Repo
}

type BarclampMap map[string]*Barclamp

type RepoMap map[string]*git.Repo

// A Build is a bundle of barclamps in a release that
// constitute an installable version of Crowbar
// with an addtional deliverable.
type Build interface {
	// The simple name of the build.  This will be something
	// like "master" or "openstack-os-build"
	Name() string
	// The full name of this build.  It is programatically
	// generated by combining the name of the release with the
	// name of the build.
	FullName() string
	// The Release this build is a member of.
	Release() Release
	// A map of the barclamps that are required for this build.
	// This does not include barclamps from this build's parent!
	Barclamps() BarclampMap
	// The parent of this build.  Every release must contain a master
	// build (which holds the core Crowbar barclamps, and has no parent),
	// and all other builds in a release are children of another build.
	Parent() Build
}

type BuildMap map[string]Build

// A Release is a stream of development for Crowbar.
// At any given time we may have multiple Releases.
// A Release consists of a collection of related builds.
type Release interface {
	// Each release must have a name.
	Name() string
	// The builds that are members of this release.
	Builds() BuildMap
	// The parent of this release.
	// We may return nil if there is no parent.
	Parent() Release
}

type ReleaseMap map[string]Release

// A Metadata allows the rest of Crowbar to know what releases and builds
// are available, and (a little later) modify it.  The reason this
// and the above are interfaces is to allow for having multiple ways to
// store the release metadata.
type Metadata interface {
	// All the releases that this metadata source knows about.
	Releases() ReleaseMap
	// All the builds that this metadata source knows about.
	AllBuilds() BuildMap
	// Test to see if Crowbar is using this metadata.
	Probe(*Crowbar) error
}

// Track the priority of a Crowbar remote.
type Remote struct {
	Priority      int
	Urlbase, Name string
}

// The master struct for managing Crowbar instances.
type Crowbar struct {
	Repo      *git.Repo
	Barclamps RepoMap
	Remotes   map[string]*Remote
	Meta      Metadata
}

// The current instance of Crowbar we operate on.
var MemoCrowbar *Crowbar

// If e is of type error, log it as a fatal error and die.
// Otherwise, don't do anything.
func dieIfError(e error) {
	if e != nil {
		log.Fatal(e)
	}
}

// Find Crowbar from the current path.
func findCrowbar(path string) (res *Crowbar, err error) {
	if MemoCrowbar != nil {
		return MemoCrowbar, nil
	}
	if path == "" {
		path, err = os.Getwd()
		dieIfError(err)
	}
	path, err = filepath.Abs(path)
	dieIfError(err)
	repo, err := git.Open(path)
	if err != nil {
		return nil, errors.New("Cannot find Crowbar")
	}
	path = repo.Path()
	parent := filepath.Dir(path)
	// If this is a raw repo, recurse and keep looking.
	if repo.IsRaw() {
		res, err = findCrowbar(parent)
		return
	}
	// See if we have something that looks like a crowbar repo here.
	stat, err := os.Stat(filepath.Join(path, "barclamps"))
	if err != nil || !stat.IsDir() {
		res, err = findCrowbar(parent)
		return
	}
	// We do.  Populate the crowbar struct.
	res = &Crowbar{
		Repo:      repo,
		Barclamps: make(map[string]*git.Repo),
		Remotes:   make(map[string]*Remote),
	}
	dirs, err := ioutil.ReadDir(filepath.Join(path, "barclamps"))
	dieIfError(err)
	// populate our list of barclamps
	for _, bc := range dirs {
		if !bc.IsDir() {
			continue
		}
		stat, err = os.Lstat(filepath.Join(path, "barclamps", bc.Name(), ".git"))
		if err != nil {
			log.Println(err)
			continue
		}
		mode := stat.Mode()
		if (mode & (os.ModeDir | os.ModeSymlink)) == 0 {
			continue
		}
		repo, err = git.Open(filepath.Join(path, "barclamps", bc.Name()))
		if err != nil {
			log.Println(err)
			continue
		}
		res.Barclamps[bc.Name()] = repo
	}
	// populate remotes next

	remotes := res.Repo.Find("crowbar.remote.")
	var rem *Remote
	for k, v := range remotes {
		parts := strings.Split(k, ".")
		if res.Remotes[parts[2]] == nil {
			rem = new(Remote)
			rem.Name = parts[2]
			rem.Priority = 50 // default.
			res.Remotes[parts[2]] = rem
		} else {
			rem = res.Remotes[parts[2]]
		}
		switch parts[3] {
		case "priority":
			p, e := strconv.Atoi(v)
			if e == nil {
				rem.Priority = p
			}
		case "urlbase":
			rem.Urlbase = v
		}
	}
	res.Meta = new(FlatMetadata)
	err = res.Meta.Probe(res)
	if err != nil {
		return nil, err
	}
	MemoCrowbar = res
	return res, nil
}

// This is the same as findCrowbar, except we die if we cannot find Crowbar.
func mustFindCrowbar(path string) *Crowbar {
	res, err := findCrowbar(path)
	dieIfError(err)
	return res
}

// Get all the releases we know about.
func (c *Crowbar) Releases() ReleaseMap {
	return c.Meta.Releases()
}

// Get all of the builds we know how to build.
func (c *Crowbar) Builds() BuildMap {
	return c.Meta.AllBuilds()
}

// Get a specific release.
func (c *Crowbar) Release(release string) Release {
	rels := c.Releases()
	res, ok := rels[release]
	if !ok {
		log.Fatalf("%s is not a release!\n", release)
	}
	return res
}

// Given the name of a release, return what its git branch should be.
func (c *Crowbar) ReleaseBranch(release string) string {
	parts := strings.Split(release, "/")
	if len(parts) == 1 {
		return "release/" + release + "/master"
	} else if len(parts) == 2 && parts[0] == "feature" {
		return "feature/" + parts[1] + "/master"
	} else {
		log.Fatalf("%s is not a valid release name!\n", release)
	}
	return ""
}

// Get all the release branches we care about, sorted by barclamp.
func (c *Crowbar) AllBarclampBranches() (res map[string][]string) {
	res = make(map[string][]string)
	for _, build := range c.Builds() {
		for _, bc := range build.Barclamps() {
			if res[bc.Name] == nil {
				res[bc.Name] = make([]string, 0, 4)
			}
			res[bc.Name] = append(res[bc.Name], bc.Branch)
		}
	}
	return
}

// Get all the barclamp repos, return them in a map whose keys are in the
// of "barclamp-" + the barclamp name.
func (c *Crowbar) AllBarclampRepos() (res RepoMap) {
	res = make(RepoMap)
	for name, bc := range c.Barclamps {
		res["barclamp-"+name] = bc
	}
	return res
}

func (c *Crowbar) AllOtherRepos() (res RepoMap) {
	res = make(RepoMap)
	res["crowbar"] = c.Repo
	return res
}

// Get all of the repositories that make up Crowbar.
func (c *Crowbar) AllRepos() (res RepoMap) {
	res = c.AllBarclampRepos()
	for k, v := range c.AllOtherRepos() {
		res[k] = v
	}
	return res
}

// The result type that all mappers in the repoMapReduce framework expect.
type resultToken struct {
	// name should be unique among all the mapreduce operations.
	// It will usually be the name of a barclamp or other repository.
	name string
	// true if the map operations results are valid, false otherwise.
	// We split this out because I expect that most operations will
	// care about rolling this up.
	ok bool
	// A function that will be called to commit this result in the
	// case that all the results are OK.
	commit func(chan<- bool)
	// A function that will be called to roll back any changes this result make.
	// It will be called if any of the mapped results were not OK.
	rollback func(chan<- bool)
	// The detailed result of an individual map function.
	// The framework will treat this as an opaque token.
	results interface{}
}

func noopCommit( c chan <- bool) { c <- true }

// Make a default resultToken.
// It pre-populates commit and rollback with functions that do nothing.
func makeResultToken() (res *resultToken) {
	res = &resultToken{
		commit:   noopCommit,
		rollback: noopCommit,
	}
	return
}

// Make commit and rollback functions for things that mess with
// the git config file.  This works by saving the contents of the
// git config file, and then discarding the saved changes or writing them out.
func configCheckpointer(r *git.Repo) (commit, rollback func(chan <- bool)) {
	configPath := filepath.Join(r.GitDir,"config")
	stat,err := os.Stat(configPath)
	if err != nil {
		log.Printf("Error stat'ing %s:\n",configPath)
		panic(err)
	}
	if !stat.Mode().IsRegular() {
		log.Panicf("Git config file %s is not a file!\n",configPath)
	}
	configContents, err := ioutil.ReadFile(configPath)
	if err != nil {
		log.Printf("Error opening %s\n",configPath)
		panic(err)
	}
	// By now we have saved the current config file contents.
		// No action for commit, we want to leave the new config alone.
		commit = noopCommit
	// On rollback, restore the old config.
	rollback = func(c chan <- bool) {
		err := ioutil.WriteFile(configPath,configContents,os.FileMode(777))
		if err != nil {
			log.Printf("Failed to restore old config file %s\n",configPath)
			c <- false
		} else {
			c <- true
		}
		r.ReloadConfig()
	}
	return commit, rollback
}

// A slice of pointers to result tokens.
type resultTokens []*resultToken

// A channel for passing result tokens around.
type resultChan chan *resultToken

// The function signature that a mapper function must have.
// string should be a unique name that should be derived from the name of a
//   repository in some way.
// *git.Repo is a pointer to a git repository structure.
// resultChan is the channel that the mapper should put its resultToken on.
// repoMapper must populate the commit and rollback functions in the resultToken,
// although they can be functions that do nothing.
type repoMapper func(string, *git.Repo, resultChan)

// The function signature that a reducer must have. It should loop over
// the values it gets from resultChan, evaluate overall success or failure,
// and return the overall success or failure along with an array of all the results.
type repoReducer func(resultChan) (bool, resultTokens)

// Perform operations in parallel across the repositories and collect the results.
// If all the results are OK, then the commit function of each resultToken is called,
// otherwise the rollback function of each resultToken is called.
func repoMapReduce(repos RepoMap, mapper repoMapper, reducer repoReducer) (ok bool, res resultTokens) {
	results := make(resultChan)
	defer close(results)
	for name, repo := range repos {
		go mapper(name, repo, results)
	}
	ok, res = reducer(results)
	crChan := make(chan bool)
	defer close(crChan)
	crOK := true
	for _, t := range res {
		if ok {
			go t.commit(crChan)
		} else {
			go t.rollback(crChan)
		}
	}
	for _, _ = range res {
		crOK = (<-crChan) && crOK
	}
	if !crOK {
		var cr string
		if ok {
			cr = "commit"
		} else {
			cr = "rollback"
		}
		log.Printf("Please email this traceback to crowbar@lists.us.dell.com\n")
		log.Panicf("Unable to %s all repoMapReduce operations!\n", cr)
	}
	return ok, res
}

// Perform a git fetch across all the repositories.
func (c *Crowbar) fetch(remotes []string) (ok bool, results resultTokens) {
	repos := c.AllRepos()
	// mapper and reducer are the functions we will
	// hand over to repoMapReduce.
	// mapper is pretty simple, and doesn't really demonstrate
	// anything useful.
	mapper := func(name string, repo *git.Repo, res resultChan) {
		tok := makeResultToken()
		ok, items := repo.Fetch(remotes)
		// Since you cannot unwind a fetch, use the default commit/rollback functions.
		tok.name, tok.ok, tok.results = name, ok, items
		res <- tok
	}
	// reducer iterates over all the results as they arrive,
	// printing status messages along the way and keeping
	// a running idea about which fetches worked.
	// It also serves to show off variable capture.
	reducer := func(vals resultChan) (bool, resultTokens) {
		ok := true
		res := make(resultTokens, len(repos), len(repos))
		for i, _ := range res {
			item := <-vals
			res[i] = item
			if item.ok {
				log.Printf("Fetched all updates for %s\n", item.name)
			} else {
				log.Printf("Failed to fetch all changes for %s:\n", item.name)
				fetch_results, cast_ok := item.results.(git.FetchMap)
				if !cast_ok {
					log.Panicf("Could not cast fetch results for %s into git.FetchMap\n", item.name)
				}
				for k, v := range fetch_results {
					if !v {
						log.Printf("\tRemote %s failed\n", k)
					}
				}
			}
			ok = ok && res[i].ok
		}
		return ok, res
	}
	// Now that all the setup is done, do it!
	ok, results = repoMapReduce(repos, mapper, reducer)
	c.updateTrackingBranches()
	return
}

func (c *Crowbar) is_clean() (ok bool, results resultTokens) {
	repos := c.AllRepos()
	mapper := func(name string, repo *git.Repo, res resultChan) {
		ok, items := repo.IsClean()
		tok := makeResultToken()
		// There is nothing to unwind or rollback when testing to see
		// if things are clean.
		tok.name, tok.ok, tok.results = name, ok, items
		res <- tok
	}
	reducer := func(vals resultChan) (bool, resultTokens) {
		ok := true
		res := make(resultTokens, len(repos), len(repos))
		for i, _ := range res {
			item := <-vals
			res[i] = item
			ok = ok && item.ok
		}
		return ok, res
	}
	ok, results = repoMapReduce(repos, mapper, reducer)
	return
}

func (c *Crowbar) currentRelease() Release {
	res, found := c.Repo.Get("crowbar.release")
	if found {
		return c.Release(res)
	}
	return nil
}

func (c *Crowbar) currentBuild() Build {
	res, found := c.Repo.Get("crowbar.build")
	if !found {
		return nil
	}
	builds := c.Builds()
	build, found := builds[res]
	if !found {
		log.Fatalf("Current build %s does not exist!", res)
	}
	return build
}

func (c *Crowbar) barclampsInBuild(build Build) BarclampMap {
	if build == nil {
		log.Panicf("Cannot get barclamps of a nil Build!")
	}
	var res BarclampMap
	if build.Parent() != nil {
		res = c.barclampsInBuild(build.Parent())
	} else {
		res = make(BarclampMap)
	}
	for k, v := range build.Barclamps() {
		res[k] = v
	}
	return res
}

func ShowCrowbar(cmd *commander.Command, args []string) {
	r := mustFindCrowbar("")
	log.Printf("Crowbar is located at: %s\n", r.Repo.Path())
}

func Fetch(cmd *commander.Command, args []string) {
	c := mustFindCrowbar("")
	ok, _ := c.fetch(nil)
	if ok {
		log.Printf("All updates fetched.\n")
		os.Exit(0)
	}
	os.Exit(1)
}

func IsClean(cmd *commander.Command, args []string) {
	c := mustFindCrowbar("")
	ok, items := c.is_clean()
	if ok {
		log.Println("All Crowbar repositories are clean.")
		os.Exit(0)
	}
	for _, item := range items {
		if !item.ok {
			log.Printf("%s is not clean:\n", item.name)
			for _, line := range item.results.(git.StatLines) {
				log.Printf("\t%s\n", line.Print())
			}
		}
	}
	os.Exit(1)
	return
}

func ShowRelease(cmd *commander.Command, args []string) {
	c := mustFindCrowbar("")
	fmt.Println(c.currentRelease().Name())
}

func ShowBuild(cmd *commander.Command, args []string) {
	c := mustFindCrowbar("")
	fmt.Println(c.currentBuild().FullName())
}

func Releases(cmd *commander.Command, args []string) {
	c := mustFindCrowbar("")
	res := make([]string, 0, 20)
	for release, _ := range c.Releases() {
		res = append(res, release)
	}
	sort.Strings(res)
	for _, release := range res {
		fmt.Println(release)
	}
}

func Builds(cmd *commander.Command, args []string) {
	c := mustFindCrowbar("")
	res := make([]string, 0, 20)
	if len(args) == 0 {
		for build, _ := range c.currentRelease().Builds() {
			res = append(res, c.currentRelease().Name()+"/"+build)
		}
	} else {
		for _, release := range args {
			for build, _ := range c.Release(release).Builds() {
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
	c := mustFindCrowbar("")
	res := make([]string, 0, 20)
	var build Build
	var found bool
	if len(args) == 0 {
		build = c.currentBuild()
	} else if len(args) == 1 {
		builds := c.Builds()
		build, found = builds[args[0]]
		if !found {
			log.Fatalln("No such build %s", args[0])
		}
	}
	for name, _ := range c.barclampsInBuild(build) {
		res = append(res, name)
	}
	sort.Strings(res)
	for _, name := range res {
		fmt.Println(name)
	}
}

func init() {
	commands.AddCommand(nil, &commander.Command{
		Run:       IsClean,
		UsageLine: "clean?",
		Short:     "Shows whether Crowbar overall is clean.",
	})
	commands.AddCommand(nil, &commander.Command{
		Run:       Releases,
		UsageLine: "releases",
		Short:     "Shows the releases available to work on.",
	})
	commands.AddCommand(nil, &commander.Command{
		Run:       BarclampsInBuild,
		UsageLine: "barclamps-in-build [build]",
		Short:     "Shows the releases available to work on.",
	})
	commands.AddCommand(nil, &commander.Command{
		Run:       Builds,
		UsageLine: "builds",
		Short:     "Shows the builds in a release or releases.",
	})
	commands.AddCommand(nil, &commander.Command{
		Run:       ShowRelease,
		UsageLine: "release",
		Short:     "Shows the current release",
	})
	commands.AddCommand(nil, &commander.Command{
		Run:       ShowBuild,
		UsageLine: "branch",
		Short:     "Shows the current branch",
	})
	commands.AddCommand(nil, &commander.Command{
		Run:       ShowCrowbar,
		UsageLine: "show",
		Short:     "Shows the location of the top level Crowbar repo",
	})
	commands.AddCommand(nil, &commander.Command{
		Run:       Fetch,
		UsageLine: "fetch",
		Short:     "Fetches updates from all remotes",
	})
	return
}

func Run() {
	commands.Run()
	return
}
