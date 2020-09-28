package filesystem

import (
	"bytes"
	"errors"
	. "github.com/franela/goblin"
	"github.com/pterodactyl/wings/config"
	"github.com/spf13/afero"
	"math/rand"
	"os"
	"strings"
	"testing"
	"unicode/utf8"
)

func Test(t *testing.T) {
	g := Goblin(t)

	config.Set(&config.Configuration{
		AuthenticationToken: "abc",
		System: config.SystemConfiguration{
			RootDirectory:     "/server",
			DiskCheckInterval: 150,
		},
	})

	rootFs := afero.NewMemMapFs()

	fs := New("/server", 0)
	fs.isTest = true

	aferoFs, _ := afero.NewBasePathFs(rootFs, "/server").(*afero.BasePathFs)
	fs.fs = aferoFs

	g.Describe("Path", func() {
		g.It("returns the root path for the instance", func() {
			g.Assert(fs.Path()).Equal("/server")
		})
	})

	g.Describe("Open", func() {
		buf := &bytes.Buffer{}

		g.It("opens a file if it exists on the system", func() {
			f, err := fs.fs.Create("test.txt")
			g.Assert(err).IsNil()
			f.Write([]byte("testing"))
			f.Close()

			err = fs.Open("test.txt", buf)
			g.Assert(err).IsNil()
			g.Assert(buf.String()).Equal("testing")
		})

		g.It("returns an error if the file does not exist", func() {
			err := fs.Open("test.txt", buf)
			g.Assert(err).IsNotNil()
			g.Assert(errors.Is(err, os.ErrNotExist)).IsTrue()
		})

		g.It("returns an error if the \"file\" is a directory", func() {
			err := fs.fs.Mkdir("test.txt", 0755)
			g.Assert(err).IsNil()

			err = fs.Open("test.txt", buf)
			g.Assert(err).IsNotNil()
			g.Assert(errors.Is(err, ErrIsDirectory)).IsTrue()
		})

		g.It("cannot open a file outside the root directory", func() {
			_, err := rootFs.Create("test.txt")
			g.Assert(err).IsNil()

			err = fs.Open("/../test.txt", buf)
			g.Assert(err).IsNotNil()
			g.Assert(strings.Contains(err.Error(), "file does not exist")).IsTrue()
		})

		g.AfterEach(func() {
			buf.Truncate(0)
			fs.fs.RemoveAll("/")
			fs.diskUsed = 0
		})
	})

	g.Describe("Open and WriteFile", func() {
		buf := &bytes.Buffer{}

		// Test that a file can be written to the disk and that the disk space used as a result
		// is updated correctly in the end.
		g.It("can create a new file", func() {
			r := bytes.NewReader([]byte("test file content"))

			g.Assert(fs.diskUsed).Equal(int64(0))

			err := fs.Writefile("test.txt", r)
			g.Assert(err).IsNil()

			err = fs.Open("test.txt", buf)
			g.Assert(err).IsNil()
			g.Assert(buf.String()).Equal("test file content")
			g.Assert(fs.diskUsed).Equal(r.Size())
		})

		g.It("can create a new file inside a nested directory with leading slash", func() {
			r := bytes.NewReader([]byte("test file content"))

			err := fs.Writefile("/some/nested/test.txt", r)
			g.Assert(err).IsNil()

			err = fs.Open("/some/nested/test.txt", buf)
			g.Assert(err).IsNil()
			g.Assert(buf.String()).Equal("test file content")
		})

		g.It("can create a new file inside a nested directory without a trailing slash", func() {
			r := bytes.NewReader([]byte("test file content"))

			err := fs.Writefile("some/../foo/bar/test.txt", r)
			g.Assert(err).IsNil()

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

		g.It("cannot write a file that exceedes the disk limits", func() {
			fs.diskLimit = 1024

			b := make([]byte, 1025)
			_, err := rand.Read(b)
			g.Assert(err).IsNil()
			g.Assert(len(b)).Equal(1025)

			r := bytes.NewReader(b)
			err = fs.Writefile("test.txt", r)
			g.Assert(err).IsNotNil()
			g.Assert(errors.Is(err, ErrNotEnoughDiskSpace)).IsTrue()
		})

		g.It("updates the total space used when a file is appended to", func() {
			fs.diskUsed = 100

			b := make([]byte, 100)
			_, _ = rand.Read(b)

			r := bytes.NewReader(b)
			err := fs.Writefile("test.txt", r)
			g.Assert(err).IsNil()
			g.Assert(fs.diskUsed).Equal(int64(200))

			// If we write less data than already exists, we should expect the total
			// disk used to be decremented.
			b = make([]byte, 50)
			_, _ = rand.Read(b)

			r = bytes.NewReader(b)
			err = fs.Writefile("test.txt", r)
			g.Assert(err).IsNil()
			g.Assert(fs.diskUsed).Equal(int64(150))
		})

		g.It("truncates the file when writing new contents", func() {
			r := bytes.NewReader([]byte("original data"))
			err := fs.Writefile("test.txt", r)
			g.Assert(err).IsNil()

			r = bytes.NewReader([]byte("new data"))
			err = fs.Writefile("test.txt", r)
			g.Assert(err).IsNil()

			err = fs.Open("test.txt", buf)
			g.Assert(err).IsNil()
			g.Assert(buf.String()).Equal("new data")
		})

		g.AfterEach(func() {
			buf.Truncate(0)
			fs.fs.RemoveAll("/")
			fs.diskUsed = 0
			fs.diskLimit = 0
		})
	})

	g.Describe("CreateDirectory", func() {
		g.It("should create missing directories automatically", func() {
			err := fs.CreateDirectory("test", "foo/bar/baz")
			g.Assert(err).IsNil()

			st, err := fs.fs.Stat("foo/bar/baz/test")
			g.Assert(err).IsNil()
			g.Assert(st.IsDir()).IsTrue()
			g.Assert(st.Name()).Equal("test")
		})

		g.It("should work with leading and trailing slashes", func() {
			err := fs.CreateDirectory("test", "/foozie/barzie/bazzy/")
			g.Assert(err).IsNil()

			st, err := fs.fs.Stat("foozie/barzie/bazzy/test")
			g.Assert(err).IsNil()
			g.Assert(st.IsDir()).IsTrue()
			g.Assert(st.Name()).Equal("test")
		})

		g.It("should not allow the creation of directories outside the root", func() {
			err := fs.CreateDirectory("test", "e/../../something")
			g.Assert(err).IsNotNil()
			g.Assert(strings.Contains(err.Error(), "file does not exist")).IsTrue()
		})

		g.It("should not increment the disk usage", func() {
			err := fs.CreateDirectory("test", "/")
			g.Assert(err).IsNil()
			g.Assert(fs.diskUsed).Equal(int64(0))
		})

		g.AfterEach(func() {
			fs.fs.RemoveAll("/")
		})
	})

	g.Describe("Rename", func() {
		g.BeforeEach(func() {
			f, err := fs.fs.OpenFile("source.txt", os.O_CREATE|os.O_TRUNC, 0644)
			if err != nil {
				panic(err)
			}
			defer f.Close()

			_, err = f.WriteString("test content")
			if err != nil {
				panic(err)
			}
		})

		g.It("returns an error if the target already exists", func() {
			fs.fs.OpenFile("target.txt", os.O_CREATE, 0644)

			err := fs.Rename("source.txt", "target.txt")
			g.Assert(err).IsNotNil()
			g.Assert(errors.Is(err, os.ErrExist)).IsTrue()
		})

		g.It("returns an error if the final destination is the root directory", func() {
			err := fs.Rename("source.txt", "/")
			g.Assert(err).IsNotNil()
			g.Assert(errors.Is(err, os.ErrExist)).IsTrue()
		})

		g.It("does not allow renaming to a location outside the root", func() {
			err := fs.Rename("source.txt", "../target.txt")
			g.Assert(err).IsNotNil()
			g.Assert(errors.Is(err, os.ErrNotExist)).IsTrue()
		})

		g.It("does not allow renaming from a location outside the root", func() {
			f, err := rootFs.OpenFile("ext-source.txt", os.O_CREATE, 0644)
			if err != nil {
				panic(err)
			}
			f.Close()

			err = fs.Rename("../ext-source.txt", "target.txt")
			g.Assert(err).IsNotNil()
			g.Assert(errors.Is(err, os.ErrNotExist)).IsTrue()
		})

		g.It("allows a file to be renamed", func() {
			err := fs.Rename("source.txt", "target.txt")
			g.Assert(err).IsNil()

			_, err = fs.fs.Stat("source.txt")
			g.Assert(err).IsNotNil()
			g.Assert(errors.Is(err, os.ErrNotExist)).IsTrue()

			st, err := fs.fs.Stat("target.txt")
			g.Assert(err).IsNil()
			g.Assert(st.Name()).Equal("target.txt")
			g.Assert(st.Size()).IsNotZero()
		})

		g.It("allows a folder to be renamed", func() {
			err := fs.fs.Mkdir("source_dir", 0755)
			g.Assert(err).IsNil()

			err = fs.Rename("source_dir", "target_dir")
			g.Assert(err).IsNil()

			_, err = fs.fs.Stat("source_dir")
			g.Assert(err).IsNotNil()
			g.Assert(errors.Is(err, os.ErrNotExist)).IsTrue()

			st, err := fs.fs.Stat("target_dir")
			g.Assert(err).IsNil()
			g.Assert(st.IsDir()).IsTrue()
		})

		g.It("returns an error if the source does not exist", func() {
			err := fs.Rename("missing.txt", "target.txt")
			g.Assert(err).IsNotNil()
			g.Assert(errors.Is(err, os.ErrNotExist)).IsTrue()
		})

		g.It("creates directories if they are missing", func() {
			err := fs.Rename("source.txt", "nested/folder/target.txt")
			g.Assert(err).IsNil()

			st, err := fs.fs.Stat("nested/folder/target.txt")
			g.Assert(err).IsNil()
			g.Assert(st.Name()).Equal("target.txt")
		})

		g.AfterEach(func() {
			fs.fs.RemoveAll("/")
		})
	})

	g.Describe("Copy", func() {
		g.BeforeEach(func() {
			f, err := fs.fs.OpenFile("source.txt", os.O_CREATE|os.O_TRUNC, 0644)
			if err != nil {
				panic(err)
			}
			defer f.Close()

			_, err = f.WriteString("test content")
			if err != nil {
				panic(err)
			}

			fs.diskUsed = int64(utf8.RuneCountInString("test content"))
		})

		g.It("should return an error if the source does not exist", func() {
			err := fs.Copy("foo.txt")
			g.Assert(err).IsNotNil()
			g.Assert(errors.Is(err, os.ErrNotExist)).IsTrue()
		})

		g.It("should return an error if the source is outside the root", func() {
			f, err := rootFs.OpenFile("ext-source.txt", os.O_CREATE, 0644)
			if err != nil {
				panic(err)
			}
			f.Close()

			err = fs.Copy("../ext-source.txt")
			g.Assert(err).IsNotNil()
			g.Assert(errors.Is(err, os.ErrNotExist)).IsTrue()
		})

		g.It("should return an error if the source directory is outside the root", func() {
			f, err := rootFs.OpenFile("nested/in/dir/ext-source.txt", os.O_CREATE, 0644)
			if err != nil {
				panic(err)
			}
			f.Close()

			err = fs.Copy("../nested/in/dir/ext-source.txt")
			g.Assert(err).IsNotNil()
			g.Assert(errors.Is(err, os.ErrNotExist)).IsTrue()

			err = fs.Copy("nested/in/../../../nested/in/dir/ext-source.txt")
			g.Assert(err).IsNotNil()
			g.Assert(errors.Is(err, os.ErrNotExist)).IsTrue()
		})

		g.It("should return an error if the source is a directory", func() {
			err := fs.fs.Mkdir("dir", 0755)
			g.Assert(err).IsNil()

			err = fs.Copy("dir")
			g.Assert(err).IsNotNil()
			g.Assert(errors.Is(err, os.ErrNotExist)).IsTrue()
		})

		g.It("should return an error if there is not space to copy the file", func() {
			fs.diskLimit = 2

			err := fs.Copy("source.txt")
			g.Assert(err).IsNotNil()
			g.Assert(errors.Is(err, ErrNotEnoughDiskSpace)).IsTrue()
		})

		g.It("should create a copy of the file and increment the disk used", func() {
			err := fs.Copy("source.txt")
			g.Assert(err).IsNil()

			_, err = fs.fs.Stat("source.txt")
			g.Assert(err).IsNil()

			_, err = fs.fs.Stat("source copy.txt")
			g.Assert(err).IsNil()
		})

		g.It("should create a copy of the file with a suffix if a copy already exists", func() {
			err := fs.Copy("source.txt")
			g.Assert(err).IsNil()

			err = fs.Copy("source.txt")
			g.Assert(err).IsNil()

			r := []string{"source.txt", "source copy.txt", "source copy 1.txt"}

			for _, name := range r {
				_, err = fs.fs.Stat(name)
				g.Assert(err).IsNil()
			}

			g.Assert(fs.diskUsed).Equal(int64(utf8.RuneCountInString("test content")) * 3)
		})

		g.It("should create a copy inside of a directory", func() {
			f, err := fs.fs.OpenFile("nested/in/dir/source.txt", os.O_CREATE|os.O_TRUNC, 0644)
			g.Assert(err).IsNil()
			f.Close()

			err = fs.Copy("nested/in/dir/source.txt")
			g.Assert(err).IsNil()

			_, err = fs.fs.Stat("nested/in/dir/source.txt")
			g.Assert(err).IsNil()

			_, err = fs.fs.Stat("nested/in/dir/source copy.txt")
			g.Assert(err).IsNil()
		})

		g.AfterEach(func() {
			fs.fs.RemoveAll("/")
			fs.diskUsed = 0
			fs.diskLimit = 0
		})
	})
}
