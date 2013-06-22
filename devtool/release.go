package devtool

import (
	"fmt"
	"github.com/VictorLowther/go-git/git"
	"log"
	"sort"
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
	if release == "development" {
		return "master"
	}
	parts := strings.Split(release, "/")
	switch {
	case len(parts) == 1:
		return "release/" + release + "/master"
	case len(parts) == 2 && parts[0] == "feature":
		return "feature/" + parts[1] + "/master"
	case len(parts) == 2 && parts[0] == "local":
		return "local/" + parts[1] + "/master"
	default:
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
	for name := range rel.Builds() {
		fmt.Printf("\t%s\n", name)
	}
}

type cherryRefs struct {
	base, working         *git.Ref
	baseName, workingName string
}

func showChanges(refs map[string]*cherryRefs) {
	names := make([]string,0,10)
	for name := range refs{
		names = append(names,name)
	}
	sort.Strings(names)
	noChanges := true
	for _, name := range names {
		r := refs[name]
		changes, err := r.working.CherryLog(r.base)
		if err != nil || (len(changes) == 0) {
			continue
		}
		noChanges = false
		fmt.Printf("\n%s: changes in %s compared to %s\n", name, r.workingName, r.baseName)
		for _, change := range changes {
			fmt.Printf("%s\n", change)
		}
	}
	if noChanges {
		fmt.Println("No unmerged changes")
	}
}

func findLocalChangeRefs(rel Release) (refs map[string]*cherryRefs) {
	barclamps := rel.Barclamps()
	refs = make(map[string]*cherryRefs)
	for name, barclamp := range barclamps {
		localRef, err := barclamp.Repo.Ref(barclamp.Branch)
		if err != nil {
			continue
		}
		baseRef, err := localRef.TrackedRef()
		if err != nil {
			continue
		}
		refs["barclamp-"+name] = &cherryRefs{
			base:        baseRef,
			working:     localRef,
			baseName:    "upstream",
			workingName: "local",
		}
	}
	return
}

// CrossReleaseChanges will find all commits in the target release that
// are not present in the base release. It uses the same logic that git-cherry uses.
func CrossReleaseChanges(target, base Release) {
	baseBarclamps,targetBarclamps := base.Barclamps(), target.Barclamps()
	commonNames := make([]string,0,10)
	for name := range baseBarclamps {
		if _,ok := targetBarclamps[name]; ok {
			commonNames = append(commonNames,name)
		}
	}
	refs := make(map[string]*cherryRefs)
	for _,name := range commonNames {
		a := baseBarclamps[name]
		b := targetBarclamps[name]
		aRef,err := a.Repo.Ref(a.Branch)
		if err != nil {
			continue
		}
		bRef,err := b.Repo.Ref(b.Branch)
		if err != nil {
			continue
		}
		refs["barclamp-"+name] = &cherryRefs{
			base: aRef,
			working: bRef,
			baseName: base.Name(),
			workingName: target.Name(),
		}
	}
	showChanges(refs)
}

// LocalChanges shows any local changes to a release that have not
// been comitted upstream.  It uses the same logic that git-cherry uses.
func LocalChanges(rel Release) {
	showChanges(findLocalChangeRefs(rel))
}

// RemoteChanges shows any remote changes to a release that have been
// fetched but not merged into the local branches.
// It uses the same logic that git-cherry uses.
func RemoteChanges(rel Release) {
	refs := findLocalChangeRefs(rel)
	for _,r := range refs {
		r.baseName, r.workingName = r.workingName, r.baseName
		r.base, r.working = r.working, r.base
	}
	showChanges(refs)
}
