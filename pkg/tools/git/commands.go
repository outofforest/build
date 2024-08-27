package git

import "github.com/outofforest/build"

// Commands is a set of commands useful for any environment.
var Commands = map[string]build.Command{
	"git/isclean": {
		Description: "Verifies that there are no uncommitted changes",
		Fn:          IsStatusClean,
	},
}
