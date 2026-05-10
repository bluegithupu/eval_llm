package repository

import (
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository interfaces are defined in their respective files
// This placeholder imports pgx to ensure dependency is tracked

// Ensure pgx types are referenced for go mod
var _ pgx.Row
var _ *pgxpool.Pool
