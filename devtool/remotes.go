package devtool

import (
	"fmt"
	"github.com/VictorLowther/go-git/git"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
)

// Something to hang methods off of for sort.Sort() to use.
type RemoteSlice []*Remote

// Return the length of the slice holding the remotes.
func (s RemoteSlice) Len() int {
	return len(s)
}

// COmapre remotes based on priority.
func (s RemoteSlice) Less(i, j int) bool {
	return s[i].Priority < s[j].Priority
}

// Swap two remotes.
func (s RemoteSlice) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

// Get the Crowbar remotes, sorted by priority.
func SortedRemotes() (res RemoteSlice) {
	res = make(RemoteSlice, 0, 2)
	for _, remote := range Remotes {
		res = append(res, remote)
	}
	sort.Sort(res)
	return res
}

// Recreate the tracking branches in the git repositories based on
// their priorities.
func UpdateTrackingBranches() (ok bool, res ResultTokens) {
	branchMap := AllBarclampBranches()
	remotes := SortedRemotes()
	log.Println("Updating local tracking branches.")
	mapper := func(name string, repo *git.Repo, res resultChan) {
		tok := makeResultToken()
		tok.commit, tok.rollback = configCheckpointer(repo)
		tok.Name, tok.OK, tok.Results = name, true, nil
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
				log.Printf("%s: %s will track %s\n", name, ref.Name(), remote.Name)
				if err := ref.TrackRemote(remote.Name); err != nil {
					log.Print(err)
					tok.OK = false
				}
				break
			}
		}
		res <- tok
	}
	ok, res = repoMapReduce(Barclamps, mapper, makeBasicReducer(len(Barclamps)))
	return
}

// Test to see if a remote name is valid.
// Currently we only allow alpha characters, which is probably too restrictive.
func validRemoteName(name string) bool {
	matcher := regexp.MustCompile("^[[:alpha:]]+$")
	if !matcher.MatchString(name) {
		log.Printf("%s is not a valid name for a remote!")
		return false
	}
	return true
}

// Check to see if the remote passed to this structure is valid.
// Currently, validity consists of:
// remote.Urlbase being a valid URL without any embedded user info.
// remote.Ulrbase starting with git, http, https, or ssh.
// remote.Name passing validRemoteName
// remote.Priority being between 1 and 100
func ValidateRemote(remote *Remote) bool {
	url, err := url.Parse(remote.Urlbase)
	if err != nil {
		log.Printf("%s is not a URL!\n", remote.Urlbase)
		return false
	} else if !url.IsAbs() {
		log.Printf("%s is not an absolute URL!\n", remote.Urlbase)
		return false
	}
	switch url.Scheme {
	case "git":
		fallthrough
	case "http":
		fallthrough
	case "https":
		if url.User != nil {
			log.Printf("Please don't embed userinfo in your http(s) or git URL!\n")
			log.Printf("Instead, modify your .netrc to include it for %s\n", url.Host)
			log.Printf("Example:\n")
			log.Printf("  machine %s login <username> password <password>\n", url.Host)
			return false
		}
	case "ssh":
		if url.User == nil {
			log.Printf("%s does not include an embedded username!", remote.Urlbase)
			return false
		}
	default:
		log.Printf("URL scheme %s is not supported by the dev tool for now.", url.Scheme)
		return false
	}
	if remote.Name == "" {
		remote.Name = filepath.Base(url.Path)
	}
	if !validRemoteName(remote.Name) {
		return false
	}
	if remote.Priority < 1 || remote.Priority > 100 {
		log.Printf("Priority must be a number between 1 and 100 (currently %d)!\n", remote.Priority)
		return false
	}
	return true
}

func addRemote(remote *Remote) {
	maybeAddRemote := func(repo *git.Repo, reponame string, remote *Remote) {
		if repo.HasRemote(remote.Name) {
			log.Printf("%s already has a repo named %s.\n", reponame, remote.Name)
			log.Printf("Will replace it.")
			repo.ZapRemote(remote.Name)
		}
		err := repo.AddRemote(remote.Name, remote.Urlbase+"/"+reponame)
		if err != nil {
			log.Printf("Error adding %s to %s:", remote.Name, reponame)
			log.Fatalln(err)
		}
	}
	for name, repo := range Barclamps {
		reponame := "barclamp-" + name
		maybeAddRemote(repo, reponame, remote)
	}
	for name, repo := range AllOtherRepos() {
		maybeAddRemote(repo, name, remote)
	}
}

// Add a new Crowbar remote to all of the repositories.
func AddRemote(remote *Remote) {
	if !ValidateRemote(remote) {
		log.Fatalf("%s failed validation.", remote.Name)
	}
	if Remotes[remote.Name] != nil {
		log.Panicf("Already have a remote named %s\n", remote.Name)
	}
	Repo.Set("crowbar.remote."+remote.Name+".priority", fmt.Sprint(remote.Priority))
	Repo.Set("crowbar.remote."+remote.Name+".urlbase", remote.Urlbase)
	addRemote(remote)
}

// Remove an already-existing Crowbar remote to all of the repositories.
func ZapRemote(remote *Remote) {
	if Remotes[remote.Name] == nil {
		log.Panicf("Remote %s already removed!\n", remote.Name)
	}
	for _, repo := range AllRepos() {
		if !repo.HasRemote(remote.Name) {
			return
		}
		_ = repo.ZapRemote(remote.Name)
	}
	Repo.Unset("crowbar.remote." + remote.Name + ".priority")
	Repo.Unset("crowbar.remote." + remote.Name + ".urlbase")
}

// Rename a remote
func RenameRemote(remote *Remote, newname string) {
	if Remotes[newname] != nil {
		log.Fatalf("Remote %s already exists, cannot rename %s to it.\n", newname, remote.Name)
	}
	if !validRemoteName(newname) {
		os.Exit(1)
	}
	for _, repo := range AllRepos() {
		_ = repo.RenameRemote(remote.Name, newname)
	}
	Repo.Unset("crowbar.remote." + remote.Name + ".priority")
	Repo.Unset("crowbar.remote." + remote.Name + ".urlbase")
	delete(Remotes, remote.Name)
	remote.Name = newname
	Remotes[remote.Name] = remote
	Repo.Set("crowbar.remote."+remote.Name+".priority", fmt.Sprint(remote.Priority))
	Repo.Set("crowbar.remote."+remote.Name+".urlbase", remote.Urlbase)
}

// Synchronize remote specifications across all the repositories.
func SyncRemotes() {
	for reponame, repo := range AllRepos() {
		remotes := repo.Remotes()
		for _, remote := range Remotes {
			repopath := filepath.Join(remote.Urlbase, reponame)
			if url, found := remotes[remote.Name]; found {
				continue
			} else if found && url != repopath {
				log.Printf("Remote %s in repo %s not pointing at proper URL.\n", remote.Name, reponame)
				repo.ZapRemote(remote.Name)
			}
			if found, _ := repo.ProbeURL(repopath); found {
				log.Printf("Adding new remote %s (%s) to %s\n", remote.Name, repopath, reponame)
				repo.AddRemote(remote.Name, repopath)
			} else {
				log.Printf("Repo %s is not at remote %s\n", reponame, remote.Name)
			}
		}
	}
}

// Set a new remote Urlbase.
func SetRemoteURLBase(remote *Remote, newurl string) {
	remote.Urlbase = newurl
	if !ValidateRemote(remote) {
		log.Fatalf("Refusing to set new URL %s for %s\n", newurl)
	}
	ZapRemote(remote)
	AddRemote(remote)
}
