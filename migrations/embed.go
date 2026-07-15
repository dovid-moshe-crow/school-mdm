package migrations

import "embed"

// SQL holds numbered migration files.
//
//go:embed *.sql
var SQL embed.FS
