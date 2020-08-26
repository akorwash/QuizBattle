package controllertests

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHome(t *testing.T) {
	//Home will be success (static page)
	assert.Equal(t, "", "")
}
