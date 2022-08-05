package airgap

import (
	"fmt"
	"sort"
)

func ValidateArchs(archs []string) error {
	for _, arch := range archs {
		if !ValidatedArch[arch] {
			return fmt.Errorf("arch %s is not validated", arch)
		}
	}
	return nil
}

func GetValidatedArchs() []string {
	var rtn []string
	for arch := range ValidatedArch {
		rtn = append(rtn, arch)
	}
	sort.Strings(rtn)
	return rtn
}

func GetArchDiff(current, target []string) (add, del []string) {
	currentMap := map[string]bool{}
	for _, arch := range current {
		currentMap[arch] = true
	}
	for _, arch := range target {
		if !currentMap[arch] {
			add = append(add, arch)
		} else {
			delete(currentMap, arch)
		}
	}
	for arch := range currentMap {
		del = append(del, arch)
	}
	return
}
