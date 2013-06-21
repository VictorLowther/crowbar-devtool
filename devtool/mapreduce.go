package devtool

import (
	"github.com/VictorLowther/go-git/git"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
)

// The result type that all mappers in the repoMapReduce framework expect.
type ResultToken struct {
	// name should be unique among all the mapreduce operations.
	// It will usually be the name of a barclamp or other repository.
	Name string
	// true if the map operations results are valid, false otherwise.
	// We split this out because I expect that most operations will
	// care about rolling this up.
	OK bool
	// A function that will be called to commit this result in the
	// case that all the results are OK.
	commit func(chan<- bool)
	// A function that will be called to roll back any changes this result make.
	// It will be called if any of the mapped results were not OK.
	rollback func(chan<- bool)
	// The detailed result of an individual map function.
	// The framework will treat this as an opaque token.
	Results interface{}
}

func noopCommit(c chan<- bool) { c <- true }

// Make a default ResultToken.
// It pre-populates commit and rollback with functions that do nothing.
func makeResultToken() (res *ResultToken) {
	res = &ResultToken{
		commit:   noopCommit,
		rollback: noopCommit,
	}
	return
}

// Make commit and rollback functions for things that mess with
// the git config file.  This works by saving the contents of the
// git config file, and then discarding the saved changes or writing them out.
func configCheckpointer(r *git.Repo) (commit, rollback func(chan<- bool)) {
	configPath := filepath.Join(r.GitDir, "config")
	stat, err := os.Stat(configPath)
	if err != nil {
		log.Printf("Error stat'ing %s:\n", configPath)
		panic(err)
	}
	if !stat.Mode().IsRegular() {
		log.Panicf("Git config file %s is not a file!\n", configPath)
	}
	configContents, err := ioutil.ReadFile(configPath)
	if err != nil {
		log.Printf("Error opening %s\n", configPath)
		panic(err)
	}
	// By now we have saved the current config file contents.
	// No action for commit, we want to leave the new config alone.
	commit = noopCommit
	// On rollback, restore the old config.
	rollback = func(c chan<- bool) {
		err := ioutil.WriteFile(configPath, configContents, os.FileMode(777))
		if err != nil {
			log.Printf("Failed to restore old config file %s\n", configPath)
			c <- false
		} else {
			c <- true
		}
		r.ReloadConfig()
	}
	return commit, rollback
}

// Make commit and rollback functions for a specific repo where we will
// be messing with the branches.
func branchCheckpointer(r *git.Repo) (commit, rollback func(chan<- bool)) {
	// We only care about branch refernces, and we only want to save
	// the SHA references to the branches.
	refs := make(map[string]string)
	for _, ref := range r.Branches() {
		refs[ref.Name()] = ref.SHA
	}
	// There is no commit action.
	commit = noopCommit
	// On rollback, force all the branches back to where we were.
	rollback = func(c chan<- bool) {
		res := true
		for name, sha := range refs {
			cmd, _, _ := r.Git("branch", "-f", name, sha)
			res = res && (cmd.Run() == nil)
		}
		c <- res

	}
	return
}

// A slice of pointers to result tokens.
type ResultTokens []*ResultToken

// A channel for passing result tokens around.
type resultChan chan *ResultToken

// The function signature that a mapper function must have.
// string should be a unique name that should be derived from the name of a
//   repository in some way.
// *git.Repo is a pointer to a git repository structure.
// resultChan is the channel that the mapper should put its ResultToken on.
// repoMapper must populate the commit and rollback functions in the ResultToken,
// although they can be functions that do nothing.
type repoMapper func(string, *git.Repo, resultChan)

// The function signature that a reducer must have. It should loop over
// the values it gets from resultChan, evaluate overall success or failure,
// and return the overall success or failure along with an array of all the results.
type repoReducer func(resultChan) (bool, ResultTokens)

// Make a basic reducer that can be used if more complicated processing
// during a reduce is not needed.
func makeBasicReducer(items int) repoReducer {
	return func(vals resultChan) (ok bool, res ResultTokens) {
		res = make(ResultTokens, items, items)
		ok = true
		for i := range res {
			item := <-vals
			ok = ok && item.OK
			res[i] = item
		}
		return
	}
}

// Perform operations in parallel across the repositories and collect the results.
// If all the results are OK, then the commit function of each ResultToken is called,
// otherwise the rollback function of each ResultToken is called.
func repoMapReduce(repos RepoMap, mapper repoMapper, reducer repoReducer) (ok bool, res ResultTokens) {
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
	for _ = range res {
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
