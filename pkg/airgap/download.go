package airgap

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"sync"

	"github.com/cnrancher/autok3s/pkg/common"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

const (
	tmpDirName        = ".tmp"
	tmpSuffix         = ".tmp"
	doneFilename      = ".done"
	versionFilename   = "version.json"
	imageListFilename = "k3s-images.txt"
	checksumBaseName  = "sha256sum"
	checksumExt       = ".txt"
	checksumFilename  = checksumBaseName + checksumExt
)

var (
	ErrVersionNotFound = errors.New("version not found")

	separator     = regexp.MustCompile(" +")
	ValidatedArch = map[string]bool{
		"arm64": true,
		"amd64": true,
		"arm":   true,
		"s390s": true,
	}
	resourceSuffixes = map[string][]string{
		"k3s":               {""},
		"k3s-airgap-images": {".tar.gz", ".tar"},
		checksumBaseName:    {checksumExt},
	}
	cancelDownloadMap = &sync.Map{}
)

type version struct {
	Version string
	Archs   []string
}

func (v *version) diff(pkg common.Package) (toAdd, toDel []string) {
	if v == nil {
		toAdd = pkg.Archs
		return
	}
	if v.Version != pkg.K3sVersion {
		toAdd = pkg.Archs
		toDel = v.Archs
		return
	}
	return GetArchDiff(v.Archs, pkg.Archs)
}

// DownloadPackage will update the package state and path for the package record
func DownloadPackage(pkg common.Package, logger logrus.FieldLogger) error {
	ctx, cancel := context.WithCancel(context.Background())
	downloader := &downloader{
		ctx:      ctx,
		pkg:      pkg,
		basePath: PackagePath(pkg.Name),
	}
	downloader.sourceURL = getSourceURL(pkg.K3sVersion)
	fields := logrus.Fields{
		"package": pkg.Name,
		"version": pkg.K3sVersion,
	}
	if logger != nil {
		downloader.logger = logger.WithFields(fields)
	} else {
		downloader.logger = logrus.WithFields(fields)
	}

	sort.Strings(downloader.pkg.Archs)
	cancelDownloadMap.Store(pkg.Name, cancel)
	defer func() {
		cancelDownloadMap.Delete(pkg.Name)
		cancel()
	}()

	return downloader.downloadPackage()
}

type downloader struct {
	ctx              context.Context
	sourceURL        string
	basePath         string
	imageListContent []byte
	pkg              common.Package
	logger           logrus.FieldLogger
}

func (d *downloader) downloadPackage() (er error) {
	version, err := versionAndBasePath(d.basePath)
	if err != nil {
		return err
	}

	toAddArchs, toDelArchs := version.diff(d.pkg)
	if len(toAddArchs) == 0 &&
		len(toDelArchs) == 0 &&
		isDone(d.basePath) {
		d.logger.Infof("the package %s is ready, skip downloading resources.", d.pkg.Name)
		if d.pkg.State != common.PackageActive {
			return updatePackageState(&d.pkg, common.PackageActive)
		}
		return nil
	}

	for _, arch := range toDelArchs {
		d.logger.Infof("removing package arch %s", arch)
		if err := os.RemoveAll(filepath.Join(d.basePath, arch)); err != nil {
			return err
		}
	}

	for _, reconcile := range []struct {
		state common.State
		f     func() error
	}{
		{
			state: common.PackageValidating,
			f:     d.validateVersion,
		},
		{f: d.writeVersion},
		{
			state: common.PackageDownloading,
			f: func() error {
				for _, arch := range d.pkg.Archs {
					if err := os.MkdirAll(filepath.Join(d.basePath, arch), 0755); err != nil {
						return err
					}
					d.logger.Infof("download %s resources", arch)
					if err := d.downloadArch(arch); err != nil {
						return err
					}
				}
				return nil
			},
		},
		{f: func() error { return done(d.basePath) }},
		{state: common.PackageVerifying, f: func() error {
			_, err := VerifyFiles(d.basePath)
			return err
		}},
	} {

		if d.ctx.Err() != nil {
			return d.ctx.Err()
		}
		if reconcile.state != "" {
			if err := updatePackageState(&d.pkg, reconcile.state); err != nil {
				return err
			}
		}
		if err := reconcile.f(); err != nil {
			if er := updatePackageState(&d.pkg, common.PackageOutOfSync); er != nil {
				d.logger.Warnf("failed to set package %s to %s state", d.pkg.Name, common.PackageOutOfSync)
			}
			return err
		}

	}

	d.pkg.FilePath = d.basePath
	if err := updatePackageState(&d.pkg, common.PackageActive); err != nil {
		return err
	}

	return err
}

func (d *downloader) downloadArch(arch string) error {
	if err := d.checkArchExists(arch); err != nil {
		return err
	}
	basePath := filepath.Join(d.basePath, arch)
	if isDone(basePath) {
		d.logger.Infof("arch %s has downloaded, skipped download process.", arch)
		return nil
	}

	archImageList := filepath.Join(basePath, imageListFilename)
	if err := os.WriteFile(archImageList, d.imageListContent, 0644); err != nil {
		return err
	}

	for basename, v := range resourceSuffixes {
		suffixes := getSuffixMapWithArchs(arch, basename, v)
		for origin, suffix := range suffixes {
			localFileName := basename + origin
			fullPath := filepath.Join(basePath, localFileName)
			// The download process is using tmp file to download. The file will be considered downloaded if the exact file exists.
			if _, err := os.Lstat(fullPath); err == nil {
				d.logger.Infof("%s resource %s exists, skip downloading", arch, basename)
				break
			}

			// resourceName is the file name online
			resourceName := basename + suffix
			d.logger.Infof("downloading %s for %s", localFileName, arch)
			err := d.download(fullPath, d.getFileURL(resourceName))
			if err != nil && err == context.Canceled {
				d.logger.Warnf("failed to download resource %s for %s because of context cancel", localFileName, arch)
				return err
			} else if err != nil {
				d.logger.Warnf("failed to download resource %s for %s, skip this resource, %v", localFileName, arch, err)
				continue
			}
			break
		}
	}

	if err := verifyArchFiles(arch, d.basePath); err != nil {
		return err
	}

	d.logger.Infof("all downloaded files are validated for %s", arch)

	if err := done(basePath); err != nil {
		return err
	}

	d.logger.Infof("k3s resource for %s downloaded.", arch)
	return nil
}

func (d *downloader) getFileURL(Filename string) string {
	return fmt.Sprintf("%s/%s", d.sourceURL, Filename)
}

// validateVersion will download k3s-images.txt to check the version exists or not.
func (d *downloader) validateVersion() error {
	sourceURL := getSourceURL(d.pkg.K3sVersion)
	downloadURL := fmt.Sprintf("%s/%s", sourceURL, imageListFilename)
	resp, err := doRequestWithCtx(d.ctx, http.MethodGet, downloadURL, nil)
	if err != nil {
		return errors.Wrapf(err, "failed to download image list of k3s version %s, this version may be not validated", d.pkg.K3sVersion)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		content, _ := io.ReadAll(resp.Body)
		d.logger.Debugf("failed to download image list resource, status code %d, data %s", resp.StatusCode, string(content))
		return ErrVersionNotFound
	}

	d.imageListContent, err = io.ReadAll(resp.Body)
	return err
}

// writeVersion will also remove .done file as we assume that creating/updating version is happening.
func (d *downloader) writeVersion() error {
	versionPath := filepath.Join(d.basePath, versionFilename)
	versionJSON := versionContent(d.pkg)
	_ = os.RemoveAll(versionPath)
	_ = os.RemoveAll(getDonePath(d.basePath))
	d.logger.Info("generating version file")
	return os.WriteFile(versionPath, versionJSON, 0644)
}

// checkArchExists will fire HEAD request to server and check response
func (d *downloader) checkArchExists(arch string) error {
	target := checksumBaseName + "-" + arch + checksumExt
	resp, err := doRequestWithCtx(d.ctx, http.MethodHead, d.getFileURL(target), nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if int(resp.StatusCode/100) != 2 {
		return fmt.Errorf("%s may not exist", arch)
	}
	return nil
}

func (d *downloader) download(file, fromURL string) error {
	resp, err := doRequestWithCtx(d.ctx, http.MethodGet, fromURL, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if int(resp.StatusCode/100) != 2 {
		content, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to download resource %s, %s", fromURL, string(content))
	}

	tmpFile := file + ".tmp"
	_ = os.RemoveAll(tmpFile)

	fp, err := os.Create(tmpFile)
	if err != nil {
		return err
	}
	defer fp.Close()

	if _, err := io.Copy(fp, resp.Body); err != nil {
		return err
	}

	return os.Rename(tmpFile, file)
}

func versionContent(pkg common.Package) []byte {
	version := version{
		Version: pkg.K3sVersion,
		Archs:   pkg.Archs,
	}
	data, _ := json.Marshal(version)
	return data
}

func versionAndBasePath(basePath string) (*version, error) {
	rtn, err := os.Lstat(basePath)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	if os.IsNotExist(err) {
		if err := os.MkdirAll(basePath, 0755); err != nil {
			return nil, err
		}
		return nil, nil
	}
	if !rtn.IsDir() {
		return nil, fmt.Errorf("package path %s must be a directory", basePath)
	}

	versionPath := filepath.Join(basePath, versionFilename)
	data, err := os.ReadFile(versionPath)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	v := version{}
	if err := json.Unmarshal(data, &v); err != nil {
		logrus.Warnf("failed to decode existing version json, assuming no version specified, %v", err)
		v.Version = ""
	} else {
		if isDone(basePath) {
			return &v, nil
		}
	}
	contents, err := os.ReadDir(basePath)
	if err != nil {
		return nil, err
	}
	for _, f := range contents {
		if f.IsDir() && ValidatedArch[f.Name()] {
			v.Archs = append(v.Archs, f.Name())
		}
	}
	return &v, nil
}

func CancelDownload(name string) error {
	f, loaded := cancelDownloadMap.Load(name)
	if !loaded {
		return fmt.Errorf("no downloader for package %s", name)
	}
	if cancel, ok := f.(context.CancelFunc); ok {
		cancel()
	}
	return nil
}
