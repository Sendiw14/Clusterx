// Package migrations embeds the forward-only SQL migrations of the control plane.
// Files are named NNNN_description.sql and are applied in version order by internal/migrate.
// An applied migration file must never be edited; its checksum is verified at every start.
package migrations

import "embed"

// FS holds every *.sql migration file in this directory.
//
//go:embed *.sql
var FS embed.FS
