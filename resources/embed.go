// Package resources embeds static assets that are not part of the web GUI
// (e.g. the donate QR code) so the compiled binary stays self-contained.
package resources

import "embed"

// FS exposes the files shipped in the resources directory.
//
//go:embed qrCode.png
var FS embed.FS
