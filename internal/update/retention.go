package update

import "sort"

// VersionsToRemove returns old slots eligible for cleanup. Current, previous,
// and newest keep versions are always protected.
func VersionsToRemove(versions []string, current, previous string, keep int) []string {
	if keep < 0 {
		keep = 0
	}
	sorted := append([]string(nil), versions...)
	sort.Strings(sorted)
	protected := map[string]bool{current: true, previous: true}
	for i := len(sorted) - 1; i >= 0 && len(protected) < keep+2; i-- {
		protected[sorted[i]] = true
	}
	var remove []string
	for _, version := range sorted {
		if !protected[version] {
			remove = append(remove, version)
		}
	}
	return remove
}
