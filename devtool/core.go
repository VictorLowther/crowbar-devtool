package devtool

import (
	"fmt"
	"github.com/VictorLowther/go-git/git"
	"github.com/gonuts/commander"
	"os"
	"strings"
	"strconv"
	"path/filepath"
	"io/ioutil"
)

type Remote struct {
	Priority int
	Urlbase string
}

type Crowbar struct {
	Repo *git.Repo
	Barclamps map[string]*git.Repo
	Remotes map[string]*Remote
}

func findCrowbar(path string) (res *Crowbar) {
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
		Repo: repo,
		Barclamps: make(map[string]*git.Repo),
		Remotes: make(map[string]*Remote),
	}
	dirs,err := ioutil.ReadDir(filepath.Join(path,"barclamps"))
	if err != nil {
		panic("Cannot read barclamps!")
	}
	// populate our list of barclamps
	for _,bc := range dirs {
		if !bc.IsDir() {
			continue
		}
		stat,err = os.Lstat(filepath.Join(path,"barclamps",bc.Name(),".git"))
		if err != nil {
			continue
		}
		mode := stat.Mode()
		if (mode&(os.ModeDir|os.ModeSymlink)) == 0 {
			continue
		}
		repo,err = git.Open(filepath.Join(path,"barclamps",bc.Name()))
		if err != nil {
			continue
		}
		res.Barclamps[bc.Name()]=repo
	}
	// populate remotes next
	cfg,err := res.Repo.Config()
	if err != nil {
		panic("Cannot get remotes info from git.")
	}
	remotes := cfg.Find("crowbar.remote.")
	var rem *Remote
	for k,v := range remotes {
		parts := strings.Split(k,".")
		if res.Remotes[parts[2]] == nil {
			rem = new(Remote)
			res.Remotes[parts[2]]=rem
		} else {
			rem = res.Remotes[parts[2]]
		}
		switch parts[3] {
		case "priority": rem.Priority, _ = strconv.Atoi(v)
		case "urlbase": rem.Urlbase = v
		}
	}
	return res
}

func ShowCrowbar(cmd *commander.Command, args []string) {
	r := findCrowbar("")
	fmt.Printf("Crowbar is located at: %s\n", r.Repo.Path())
	fmt.Printf("It knows about the following barclamps:\n")
	for k,_ := range r.Barclamps {
		fmt.Printf("\t%s\n",k)
	}
	fmt.Printf("It has the following remotes:\n")
	for k,_ := range r.Remotes {
		fmt.Printf("\t%s\n",k)
	}
}
