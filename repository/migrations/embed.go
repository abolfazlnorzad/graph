package migrations

import (
	"embed"

	"github.com/abolfazlnorzad/graph/pkg/migration"
)

//go:embed *.sql
var FS embed.FS

func init() {
	migration.Register(FS)
}
