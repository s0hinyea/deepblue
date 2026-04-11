// internal/db/watcher.go
//
// WatchReports has moved to internal/services/watcher.go to avoid an import
// cycle: services imports db, so db cannot also import services.
package db
