package devtool

import (
	"fmt"
	"github.com/VictorLowther/go-git/git"
	"github.com/gonuts/commander"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type Remote struct {
	Priority int
	Urlbase  string
}

type Crowbar struct {
	Repo      *git.Repo
	Barclamps map[string]*git.Repo
	Remotes   map[string]*Remote
}

var MemoCrowbar *Crowbar

func findCrowbar(path string) (res *Crowbar) {
	if MemoCrowbar != nil {
		return MemoCrowbar
	}
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
	repo, err := git.Open(path)
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
	// We do.  Populate the crowbar struct.
	res = &Crowbar{
		Repo:      repo,
		Barclamps: make(map[string]*git.Repo),
		Remotes:   make(map[string]*Remote),
	}
	dirs, err := ioutil.ReadDir(filepath.Join(path, "barclamps"))
	if err != nil {
		panic("Cannot read barclamps!")
	}
	// populate our list of barclamps
	for _, bc := range dirs {
		if !bc.IsDir() {
			continue
		}
		stat, err = os.Lstat(filepath.Join(path, "barclamps", bc.Name(), ".git"))
		if err != nil {
			continue
		}
		mode := stat.Mode()
		if (mode & (os.ModeDir | os.ModeSymlink)) == 0 {
			continue
		}
		repo, err = git.Open(filepath.Join(path, "barclamps", bc.Name()))
		if err != nil {
			continue
		}
		res.Barclamps[bc.Name()] = repo
	}
	// populate remotes next
	cfg, err := res.Repo.Config()
	if err != nil {
		panic("Cannot get remotes info from git.")
	}
	remotes := cfg.Find("crowbar.remote.")
	var rem *Remote
	for k, v := range remotes {
		parts := strings.Split(k, ".")
		if res.Remotes[parts[2]] == nil {
			rem = new(Remote)
			res.Remotes[parts[2]] = rem
		} else {
			rem = res.Remotes[parts[2]]
		}
		switch parts[3] {
		case "priority":
			rem.Priority, _ = strconv.Atoi(v)
		case "urlbase":
			rem.Urlbase = v
		}
	}
	MemoCrowbar = res
	return res
}

func ShowCrowbar(cmd *commander.Command, args []string) {
	r := findCrowbar("")
	fmt.Printf("Crowbar is located at: %s\n", r.Repo.Path())
	fmt.Printf("It knows about the following barclamps:\n")
	for k, _ := range r.Barclamps {
		fmt.Printf("\t%s\n", k)
	}
	fmt.Printf("It has the following remotes:\n")
	for k, _ := range r.Remotes {
		fmt.Printf("\t%s\n", k)
	}
}

func (c *Crowbar) fetch(remotes []string) (ok bool) {
	type tok struct {
		name    string
		ok      bool
		results git.FetchMap
	}
	ok = true
	fetches := len(c.Barclamps) + 1
	results := make([]tok, 0, fetches)
	ch := make(chan tok)
	fetcher := func(name string, repo *git.Repo) {
		ok, items := repo.Fetch(remotes)
		ch <- tok{
			name:    name,
			ok:      ok,
			results: items,
		}
	}
	go fetcher("Crowbar", c.Repo)
	for k, v := range c.Barclamps {
		go fetcher(k, v)
	}
	for {
		result := <-ch
		ok = ok && result.ok
		results = append(results, result)
		if result.ok {
			fmt.Printf("Fetched all changes for %s\n", result.name)
		} else {
			fmt.Printf("Failed to fetch all changes for %s:\n", result.name)
			for k, v := range result.results {
				if !v {
					fmt.Printf("\tRemote %s failed\n", k)
				}
			}
		}
		if len(results) == fetches {
			break
		}
	}
	close(ch)
	return ok
}

func Fetch(cmd *commander.Command, args []string) {
	c := findCrowbar("")
	if c.fetch(nil) {
		fmt.Printf("All updates fetched.\n")
		os.Exit(0)
	}
	os.Exit(1)
}
