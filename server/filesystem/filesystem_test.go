package filesystem

import (
	"bytes"
	"errors"
	. "github.com/franela/goblin"
	"github.com/pterodactyl/wings/config"
	"github.com/spf13/afero"
	"os"
	"strings"
	"testing"
)

func Test(t *testing.T) {
	g := Goblin(t)

	config.Set(&config.Configuration{
		AuthenticationToken: "abc",
		System: config.SystemConfiguration{
			RootDirectory: "/server",
			DiskCheckInterval: 150,
		},
	})

	fs := New("/server", 0)
	fs.isTest = true

	aferoFs, _ := afero.NewBasePathFs(afero.NewMemMapFs(), "/server").(*afero.BasePathFs)
	fs.fs = aferoFs

	g.Describe("Open", func() {
		g.It("opens a file if it exists on the system", func() {
			f, err := fs.fs.Create("test.txt")
			g.Assert(err).IsNil()
			f.Write([]byte("testing"))
			f.Close()

			buf := &bytes.Buffer{}
			err = fs.Open("test.txt", buf)
			g.Assert(err).IsNil()
			g.Assert(buf.String()).Equal("testing")
		})

		g.It("returns an error if the file does not exist", func() {
			buf := &bytes.Buffer{}

			err := fs.Open("test.txt", buf)
			g.Assert(err).IsNotNil()
			g.Assert(errors.Is(err, os.ErrNotExist)).IsTrue()
		})

		g.It("returns an error if the \"file\" is a directory", func() {
			buf := &bytes.Buffer{}

			err := fs.fs.Mkdir("test.txt", 0755)
			g.Assert(err).IsNil()

			err = fs.Open("test.txt", buf)
			g.Assert(err).IsNotNil()
			g.Assert(errors.Is(err, ErrIsDirectory)).IsTrue()
		})

		g.AfterEach(func() {
			fs.fs.RemoveAll("/")
			fs.diskUsed = 0
		})
	})

	g.Describe("Open and WriteFile", func() {
		// Test that a file can be written to the disk and that the disk space used as a result
		// is updated correctly in the end.
		g.It("can create a new file", func() {
			r := bytes.NewReader([]byte("test file content"))

			g.Assert(fs.diskUsed).Equal(int64(0))

			err := fs.Writefile("test.txt", r)
			g.Assert(err).IsNil()

			buf := &bytes.Buffer{}
			err = fs.Open("test.txt", buf)
			g.Assert(err).IsNil()
			g.Assert(buf.String()).Equal("test file content")
			g.Assert(fs.diskUsed).Equal(r.Size())
		})

		g.It("can create a new file inside a nested directory with leading slash", func() {
			r := bytes.NewReader([]byte("test file content"))

			err := fs.Writefile("/some/nested/test.txt", r)
			g.Assert(err).IsNil()

			buf := &bytes.Buffer{}
			err = fs.Open("/some/nested/test.txt", buf)
			g.Assert(err).IsNil()
			g.Assert(buf.String()).Equal("test file content")
		})

		g.It("can create a new file inside a nested directory without a trailing slash", func() {
			r := bytes.NewReader([]byte("test file content"))

			err := fs.Writefile("some/../foo/bar/test.txt", r)
			g.Assert(err).IsNil()

			buf := &bytes.Buffer{}
			err = fs.Open("foo/bar/test.txt", buf)
			g.Assert(err).IsNil()
			g.Assert(buf.String()).Equal("test file content")
		})

		g.It("cannot create a file outside the root directory", func() {
			r := bytes.NewReader([]byte("test file content"))

			err := fs.Writefile("/some/../foo/../../test.txt", r)
			g.Assert(err).IsNotNil()
			g.Assert(strings.Contains(err.Error(), "file does not exist")).IsTrue()
		})

		g.AfterEach(func() {
			fs.fs.RemoveAll("/")
			fs.diskUsed = 0
		})
	})
}
