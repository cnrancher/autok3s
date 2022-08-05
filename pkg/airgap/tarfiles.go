package airgap

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

func SaveToTmp(path, name string) (string, error) {
	rtn := TempDir(name)
	if err := os.MkdirAll(rtn, 0755); err != nil {
		return rtn, err
	}
	if _, err := os.Lstat(path); err != nil {
		return "", err
	}

	fp, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer fp.Close()
	gzReader, err := gzip.NewReader(fp)
	if err != nil {
		return "", err
	}
	defer gzReader.Close()
	tarReader := tar.NewReader(gzReader)
	for {
		tr, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}
		fullpath := filepath.Join(rtn, tr.Name)
		switch tr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(fullpath, 0755); err != nil {
				return "", err
			}
		case tar.TypeReg:
			parent := filepath.Dir(fullpath)
			if _, err := os.Lstat(parent); err != nil {
				if err := os.MkdirAll(parent, 0755); err != nil {
					return "", err
				}
			}
			if err := func(header *tar.Header) error {
				outFile, err := os.OpenFile(fullpath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, header.FileInfo().Mode())
				if err != nil {
					return err
				}
				defer outFile.Close()
				if _, err := io.Copy(outFile, tarReader); err != nil {
					return err
				}
				return nil
			}(tr); err != nil {
				return "", err
			}
		default:
			// ignore unknown type.
		}
	}
	return rtn, nil
}

func TarAndGzip(from, to string) error {
	_, err := os.Lstat(to)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if err == nil {
		return fmt.Errorf("file %s exists, stop exporting", to)
	}

	// related path to real path
	files := map[string]string{}
	if err := filepath.Walk(from, func(path string, info fs.FileInfo, err error) error {
		if from == path || info.IsDir() {
			return nil
		}
		f, _ := filepath.Rel(from, path)

		files[f] = path
		return nil
	}); err != nil {
		return err
	}
	toFile, err := os.Create(to)
	if err != nil {
		return err
	}
	defer toFile.Close()
	return createArchive(files, toFile)
}

func createArchive(files map[string]string, buf io.Writer) error {
	// Create new Writers for gzip and tar
	// These writers are chained. Writing to the tar writer will
	// write to the gzip writer which in turn will write to
	// the "buf" writer
	gw := gzip.NewWriter(buf)
	defer gw.Close()
	tw := tar.NewWriter(gw)
	defer tw.Close()

	// Iterate over files and add them to the tar archive
	for filename, filepath := range files {
		err := addToArchive(tw, filename, filepath)
		if err != nil {
			return err
		}
	}

	return nil
}

func addToArchive(tw *tar.Writer, filename, path string) error {
	// Open the file which will be written into the archive
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	// Get FileInfo about our file providing file size, mode, etc.
	info, err := file.Stat()
	if err != nil {
		return err
	}

	// Create a tar Header from the FileInfo data
	header, err := tar.FileInfoHeader(info, info.Name())
	if err != nil {
		return err
	}

	// Use full path as name (FileInfoHeader only takes the basename)
	// If we don't do this the directory strucuture would
	// not be preserved
	// https://golang.org/src/archive/tar/common.go?#L626
	header.Name = filename

	// Write file header to the tar archive
	err = tw.WriteHeader(header)
	if err != nil {
		return err
	}

	// Copy file content to tar archive
	_, err = io.Copy(tw, file)
	if err != nil {
		return err
	}

	return nil
}
