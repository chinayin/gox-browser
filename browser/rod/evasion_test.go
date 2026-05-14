package rod

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEvasionScript_NonEmpty(t *testing.T) {
	assert.NotEmpty(t, evasionScript, "evasion script should not be empty")
	assert.Contains(t, evasionScript, "window.chrome", "evasion script should patch window.chrome")
	assert.Contains(t, evasionScript, "chrome.runtime", "evasion script should patch chrome.runtime")
	assert.Contains(t, evasionScript, "navigator.connection", "evasion script should patch navigator.connection")
	assert.Contains(t, evasionScript, "HeadlessChrome", "evasion script should handle HeadlessChrome UA replacement")
}
