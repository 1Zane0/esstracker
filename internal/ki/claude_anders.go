//go:build !windows

package ki

import "os/exec"

// claudePfad sucht die CLI im PATH.
func claudePfad() (string, error) { return exec.LookPath("claude") }

// ohneKonsole tut hier nichts — siehe claude_windows.go.
func ohneKonsole(*exec.Cmd) {}
