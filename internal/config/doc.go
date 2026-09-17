// Package config loads, validates, and normalizes runtime configuration.
//
// Configuration precedence is: generated JSON defaults, the selected JSON
// file, then environment variables. Callers receive a fully validated Config;
// other packages must not read environment variables directly.
package config
