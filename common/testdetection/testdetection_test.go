package testdetection_test

import (
	"testing"

	"github.com/canonical/ubuntu-pro-for-wsl/common/testdetection"
	"github.com/stretchr/testify/require"
)

func TestMustBeTestingInTests(t *testing.T) {
	t.Parallel()

	defer func() {
		r := recover()
		require.Nil(t, r, "MustBeTesting should not panic as we are running in tests")
	}()

	testdetection.MustBeTesting()
}
