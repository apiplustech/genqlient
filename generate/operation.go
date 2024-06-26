package generate

import "github.com/vektah/gqlparser/v2/ast"

// Wrapper for HasFile
type OperationDefinition struct {
	ast.OperationDefinition
	HasFile bool
}
