package lucky

import (
	"os"
	"path/filepath"
	"testing"
)

// writeStubOp provides a fake `op` so the CLI backend can be selected and
// exercised with no 1Password present.
func writeStubOp(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "op")
	script := "#!/bin/sh\ncase \"$1 $2\" in\n" +
		"'vault list') echo '[{\"id\":\"v1\",\"name\":\"wtp\",\"items\":3}]';;\n" +
		"*) echo '[]';;\nesac\n"
	if err := os.WriteFile(path, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	return path
}
