// Package migrations embeds the SQL schema migrations into the binaries, so a
// deployed binary always carries a schema matching its code.
package migrations

import "embed"

// FS contains every *.sql migration file of the migrations directory.
//
//go:embed *.sql
var FS embed.FS
