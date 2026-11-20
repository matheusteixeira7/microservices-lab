// Package migrations embeds the SQL migrations into the binary, so the
// `estoque migrate` command needs no files on disk.
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
