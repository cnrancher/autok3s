package airgap

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetDataPath(t *testing.T) {
	type testcase struct {
		name       string
		args       string
		expectPath string
	}
	for _, c := range []testcase{
		{name: "no extra args", expectPath: defaultDataDirPath},
		{name: "no data path args", args: "--bind-address=0.0.0.0", expectPath: defaultDataDirPath},
		{name: "data dir args", args: "--data-dir /data", expectPath: "/data"},
		{name: "data dir args with equal sign", args: "--data-dir=/data", expectPath: "/data"},
		{name: "data dir args with short name", args: "-d /data", expectPath: "/data"},
		{name: "data dir args with short name and equal sign", args: "-d=/data", expectPath: "/data"},
		{name: "wrong data dir args", args: "--data-dir", expectPath: defaultDataDirPath},
	} {
		path := getDataPath(c.args)
		assert.Equalf(t, c.expectPath, path, "test: %s failed", c.name)
	}
}
