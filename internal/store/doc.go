// Package store provides persistence ports and their GORM implementation.
//
// Store and RoomStore are application-facing interfaces. GORMStore is the
// infrastructure adapter for both SQLite and MySQL and owns schema migration.
package store
