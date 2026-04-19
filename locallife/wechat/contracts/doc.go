// Package contracts stores stable WeChat request and response contract structs.
//
// The migration strategy is:
// 1. Define the canonical contract type here.
// 2. Keep compatibility aliases in the parent wechat package while callers migrate.
// 3. Group files by capability group or interface instead of building one giant struct file.
package contracts
