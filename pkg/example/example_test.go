package example

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExample_Hello(t *testing.T) {
	want := "Hello, Testing"
	got := Hello("Testing")
	assert.Equal(t, want, got)
}
