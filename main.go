// genqlient is a GraphQL client generator for Go.
//
// To run genqlient:
//
//	go run github.com/apiplustech/genqlient
//
// For programmatic access, see the "generate" package, below.  For
// user documentation, see the project [GitHub].
//
// [GitHub]: https://github.com/apiplustech/genqlient
package main

import (
	"github.com/apiplustech/genqlient/generate"
)

func main() {
	generate.Main()
}
