package devtool

import (
	"fmt"
	"github.com/VictorLowther/go-git/git"
	"log"
	"strings"
)

// Get all the releases we know about.
func Releases() ReleaseMap {
	return Meta.Releases()
}

// Get a specific release.
func GetRelease(release string) Release {
	rels := Releases()
	res, ok := rels[release]
	if !ok {
		log.Fatalf("%s is not a release!\n", release)
	}
	return res
}

func SplitRelease(from Release, to string) (res Release, err error) {
	releases := Releases()
	if _, found := releases[to]; found {
		return nil, fmt.Errorf("Release %s already exists, cannot create it!\n", to)
	}
	newBranch := ReleaseBranch(to)
	barclamps := from.Barclamps()
	bases := make([]*git.Ref, 0, len(barclamps))
	// Get all the refs we need to fork, or die.
	for _, barclamp := range barclamps {
		base, err := barclamp.Repo.Ref(barclamp.Branch)
		if err != nil {
			return nil, fmt.Errorf("Base ref %s for barclamp %s in source release %s does not exist!", from.Name(), barclamp.Name, barclamp.Branch)
		}
		if _, err = barclamp.Repo.Ref(newBranch); err == nil {
			return nil, fmt.Errorf("%s already has a ref named %s", barclamp.Name, newBranch)
		}
		bases = append(bases, base)
	}
	// By now, all our base in all our barclamp are belong to us.
	// Create new branches based on the base ref.
	for _, base := range bases {
		if _, err = base.Branch(newBranch); err != nil {
			return nil, err
		}
	}
	res, err = from.FinalizeSplit(to, newBranch)
	return
}

// Given the name of a release, return what its git branch should be.
func ReleaseBranch(release string) string {
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

// Get the current release that this repo set is working in.
func CurrentRelease() Release {
	res, found := Repo.Get("crowbar.release")
	if found {
		return GetRelease(res)
	}
	return nil
}
