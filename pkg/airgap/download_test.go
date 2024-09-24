package airgap

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCheckHash(t *testing.T) {
	f, err := os.CreateTemp(".", "test-checksum-****")
	if err != nil {
		t.Fatal(err)
	}
	filename := f.Name()
	defer func() {
		os.RemoveAll(filename)
	}()

	if _, err := f.WriteString("abcd\n"); err != nil {
		t.Fatal(err)
	}
	f.Close()
	targetHash := "fc4b5fd6816f75a7c81fc8eaa9499d6a299bd803397166e8c4cf9280b801d62c"
	ok, err := checkFileHash(filename, targetHash)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("target Hash misatch")
	}
}

func TestDone(t *testing.T) {
	basepath := "."
	if isDone(basepath) {
		t.Fatal("return done before executing doneFunc.")
	}
	if err := done(basepath); err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(getDonePath(basepath))
	if !isDone(basepath) {
		t.Fatal("return not done after executed doneFunc.")
	}
}

func TestGetExt(t *testing.T) {
	toTest := "abc.tar.gz"
	targetName := "abc"
	targetExt := ".tar.gz"
	name, ext := getExt(toTest)
	assert.Equal(t, targetName, name)
	assert.Equal(t, targetExt, ext)
}

func TestSuffixWithArch(t *testing.T) {
	type testcase struct {
		arch     string
		basename string
		suffix   []string
		target   map[string]string
	}
	for _, c := range []testcase{
		{
			arch:     "amd64",
			basename: "k3s",
			suffix:   []string{""},
			target:   map[string]string{"": ""},
		},
		{
			arch:     "arm64",
			basename: "k3s",
			suffix:   []string{""},
			target:   map[string]string{"": "-arm64"},
		},
		{
			arch:     "arm64",
			basename: "k3s-airgap-images",
			suffix:   []string{".tar.gz", ".tar"},
			target:   map[string]string{".tar.gz": "-arm64.tar.gz", ".tar": "-arm64.tar"},
		},
	} {
		rtn := getSuffixMapWithArchs(c.arch, c.basename, c.suffix)
		assert.Equal(t, c.target, rtn)
	}
}
