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
		if release == "development" {
			return "master"
		}
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

// Remove a release.  Has no warnings or sanity checking.
func RemoveRelease(rel Release) error {
	if rel.Name() == CurrentRelease().Name() {
		return fmt.Errorf("Cannot remove current release %s", rel.Name())
	}
	for _, barclamp := range rel.Barclamps() {
		cmd, _, _ := barclamp.Repo.Git("branch", "-D", barclamp.Branch)
		if cmd.Run() != nil {
			return fmt.Errorf("Failed to remove release branch %s from %s", barclamp.Branch, barclamp.Name)
		}
	}
	return rel.Zap()
}

// Shows some useful information about a release.
func ShowRelease(rel Release) {
	fmt.Printf("Release: %s\n", rel.Name())
	parent := rel.Parent()
	if parent != nil {
		fmt.Printf("Parent: %s\n", parent.Name())
	}
	fmt.Printf("Default Branch: %s\n", ReleaseBranch(rel.Name()))
	fmt.Printf("Builds:\n")
	for name,_ := range rel.Builds() {
		fmt.Printf("\t%s\n",name)
	}
}
