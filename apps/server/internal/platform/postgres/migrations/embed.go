package migrations

import "embed"

// FS contains the PostgreSQL migrations compiled into the server binary.
//
//go:embed *.sql
var FS embed.FS
