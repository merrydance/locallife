package errorcodes

// Package errorcodes centralizes WeChat official error-code truth used by the
// payment domain.
//
// Canonical constants in this package follow the active official audit and
// standards docs. Any historical or upstream-compatible spellings still
// accepted by LocalLife must normalize back to those canonical official codes
// here instead of reappearing as ad-hoc string comparisons in callers.
