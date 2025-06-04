package version

import "fmt"

var (
	RELEASE = "UNKNOWN"
	REPO    = "UNKNOWN"
	COMMIT  = "UNKNOWN"
)

func String() string {
	return fmt.Sprintf(`
Aegis Controller
  Release:	%v
  Build:	%v
  Repository:	%v
	`, RELEASE, REPO, COMMIT)
}
