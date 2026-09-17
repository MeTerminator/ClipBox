// Package upload manages resumable, content-verified files on local disk.
//
// A session is keyed by the expected SHA-1, stores chunks in a temporary
// directory, and becomes visible in the permanent file directory only after
// size and digest verification succeeds.
package upload
