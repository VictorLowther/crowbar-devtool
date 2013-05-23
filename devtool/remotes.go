package devtool

import (
	"fmt"
	"github.com/VictorLowther/crowbar-devtool/commands"
	"github.com/VictorLowther/go-git/git"
	"github.com/gonuts/commander"
	"net/url"
	"log"
	"os"
	"regexp"
	"path/filepath"
	"sort"
	"strconv"
)

// Something to hang methods off of for sort.Sort() to use.
type RemoteSlice []*Remote

func (s RemoteSlice) Len() int {
	return len(s)
}

func (s RemoteSlice) Less(i, j int) bool {
	return s[i].Priority < s[j].Priority
}

func (s RemoteSlice) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

func (c *Crowbar) SortedRemotes() (res RemoteSlice) {
	res = make(RemoteSlice, 0, 2)
	for _, remote := range c.Remotes {
		res = append(res, remote)
	}
	sort.Sort(res)
	return res
}

func (c *Crowbar) updateTrackingBranches() {
	branchMap := c.AllBarclampBranches()
	remotes := c.SortedRemotes()
	log.Println("Updating local tracking branches.")
	mapper := func(name string, repo *git.Repo, res resultChan) {
		branches := branchMap[name]
		for _, br := range branches {
			ref, err := repo.Ref(br)
			// Does this branch actually exist?
			if err != nil || !ref.IsLocal() {
				continue
			}
			// It does, so check its remotes.
			for _, remote := range remotes {
				// Does this repo have this remote?
				if !repo.HasRemote(remote.Name) {
					continue
				}
				// It does. Do we track it?
				if r, _ := ref.Tracks(); r == remote.Name {
					break
				}
				// We do not.
				// Is there a matching remote ref for this branch?
				if !ref.HasRemoteRef(remote.Name) {
					continue
				}
				// There is one, and we will track it.
				log.Printf("%s: %s will track  %s\n", name, ref.Name(), remote.Name)
				_ = ref.TrackRemote(remote.Name)
				break
			}
		}
		res <- &resultToken{name: name, ok: true, results: nil}
	}
	reducer := func(tokens resultChan) (ok bool, res resultTokens) {
		res = make(resultTokens, len(c.Barclamps), len(c.Barclamps))
		ok = true
		for i, _ := range res {
			item := <-tokens
			res[i] = item
			ok = ok && item.ok
		}
		return
	}
	repoMapReduce(c.Barclamps, mapper, reducer)
	return
}

func validateRemote(remote *Remote) (bool) {
	url,err := url.Parse(remote.Urlbase)
	matcher := regexp.MustCompile("^[[:alpha:]]+$")
	if err != nil {
		log.Printf("%s is not a URL!\n",remote.Urlbase)
		return false
	} else if !url.IsAbs() {
		log.Printf("%s is not an absolute URL!\n",remote.Urlbase)
		return false
	}
	switch url.Scheme {
	case "git": fallthrough
	case "http": fallthrough
	case "https":
		if url.User != nil {
			log.Printf("Please don't embed userinfo in your http(s) or git URL!\n")
			log.Printf("Instead, modify your .netrc to include it for %s\n",url.Host)
			log.Printf("Example:\n")
			log.Printf("  machine %s login <username> password <password>\n",url.Host)
			return false
		}
	case "ssh":
		if url.User == nil {
			log.Printf("%s does not include an embedded username!",remote.Urlbase)
			return false
		}
	default: log.Printf("URL scheme %s is not supported by the dev tool for now.",url.Scheme)
		return false
	}
	if remote.Name == "" {
		remote.Name = filepath.Base(url.Path)
	}
	if !matcher.MatchString(remote.Name) {
		log.Printf("'%s' is not a valid name for a remote!\n",remote.Name)
		return false
	}
	if remote.Priority < 1 || remote.Priority > 100 {
		log.Printf("Priority must be a number between 1 and 100 (currently %d)!\n",remote.Priority)
		return false
	}
	return true
}

func (c *Crowbar) AddRemote(remote *Remote) {
	if c.Remotes[remote.Name] != nil {
		log.Panicf("Already have a remote named%s\n",remote.Name)
	}
	c.Repo.Set("crowbar.remote."+remote.Name+".priority",fmt.Sprint(remote.Priority))
	c.Repo.Set("crowbar.remote."+remote.Name+".urlbase",remote.Urlbase)
	maybeAddRemote := func (repo *git.Repo, reponame string, remote *Remote) {
		if repo.HasRemote(remote.Name) {
			log.Printf("%s already has a repo named %s.\n",reponame,remote.Name)
			log.Printf("Will replace it.")
			repo.ZapRemote(remote.Name)
		}
		err := repo.AddRemote(remote.Name,remote.Urlbase + "/" + reponame)
		if err != nil {
			log.Printf("Error adding %s to %s:",remote.Name,reponame)
			log.Fatalln(err)
		}
	}
	for name,repo := range c.Barclamps {
		reponame := "barclamp-" + name
		maybeAddRemote(repo,reponame,remote)
	}
	for name,repo := range c.AllOtherRepos() {
		maybeAddRemote(repo,name,remote)
	}
}

func AddRemote(cmd *commander.Command, args []string) {
	remote := &Remote{Priority: 50}
	switch len(args)  {
	case 1: remote.Urlbase = args[0]
	case 2: pri,err := strconv.Atoi(args[1])
		if err == nil {
			remote.Priority = pri
			remote.Urlbase = args[0]
		} else {
			remote.Name, remote.Urlbase = args[0],args[1]
		}
	case 3: remote.Name,remote.Urlbase = args[0],args[1]
		pri,err := strconv.Atoi(args[2])
		if err == nil {
			remote.Priority = pri
		} else {
			log.Fatalf("Last argument must be a number, but you passed %v\n",args[2])
		}
	default: log.Fatalf("Adding a remote takes at least 1 and most 3 parameters!")
	}
	if !validateRemote(remote) {
		log.Fatalf("%s failed validation.",remote.Name)
	}
	c := mustFindCrowbar("")
	if c.Remotes[remote.Name] != nil {
		log.Fatalf("%s is already a Crowbar remote.",remote.Name)
	}
	c.AddRemote(remote)
	os.Exit(0)
}

func UpdateTracking(cmd *commander.Command, args []string) {
	c := mustFindCrowbar("")
	c.updateTrackingBranches()
	os.Exit(0)
}

func ListRemotes(cmd *commander.Command, args []string) {
	c := mustFindCrowbar("")
	for _, remote := range c.SortedRemotes() {
		fmt.Printf("%s: urlbase=%s, priority=%d\n", remote.Name, remote.Urlbase, remote.Priority)
	}
	os.Exit(0)
}

func ShowRemote(cmd *commander.Command, args []string) {
	c := mustFindCrowbar("")
	if len(args) != 1 {
		log.Fatal("Need exactly 1 argument.")
	}
	remote, found := c.Remotes[args[0]]
	if !found {
		log.Fatalf("%s is not a remote!\n", args[0])
	}
	fmt.Printf("Remote %s:\n\tUrlbase: %s\n\tPriority: %d\n", remote.Name, remote.Urlbase, remote.Priority)
	os.Exit(0)
}

func init() {
	remote := commands.AddSubCommand(nil, &commander.Commander{
		Name:  "remote",
		Short: "Subcommands dealing with remote manipulation",
	})
	commands.AddCommand(remote, &commander.Command{
		Run:       UpdateTracking,
		UsageLine: "retrack",
		Short:     "Update tracking references for all branches across all releases.",
	})
	commands.AddCommand(remote, &commander.Command{
		Run:       ListRemotes,
		UsageLine: "list",
		Short:     "List the remotes Crowbar is configured to use.",
	})
	commands.AddCommand(remote, &commander.Command{
		Run:       ShowRemote,
		UsageLine: "show [remote]",
		Short:     "Show details about a specific remote",
	})
	commands.AddCommand(remote, &commander.Command{
		Run:       AddRemote,
		UsageLine: "add [remote] [URL] [priority]",
		Short:     "Add a new remote",
	})

}
