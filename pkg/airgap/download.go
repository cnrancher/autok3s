package airgap

import (
	"bufio"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/cnrancher/autok3s/pkg/common"
	"github.com/cnrancher/autok3s/pkg/settings"

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
	packagePath        = filepath.Join(common.CfgPath, "package")
	packageTmpBasePath = filepath.Join(packagePath, tmpDirName)
	downloadSourceMap  = map[string]string{
		"github":    "https://github.com/k3s-io/k3s/releases/download",
		"aliyunoss": "https://rancher-mirror.oss-cn-beijing.aliyuncs.com/k3s",
	}
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

func NewDownloader(pkg common.Package) (*Downloader, error) {
	d := &Downloader{
		pkg:      pkg,
		basePath: PackagePath(pkg.Name),
		source:   settings.PackageDownloadSource.Get(),
	}

	versionPath := d.pkg.K3sVersion
	if d.source == "aliyunoss" {
		versionPath = strings.ReplaceAll(versionPath, "+", "-")
	}
	versionPath = url.QueryEscape(versionPath)
	baseURL := downloadSourceMap[d.source]
	d.sourceURL = fmt.Sprintf("%s/%s", baseURL, versionPath)

	d.logger = logrus.WithFields(logrus.Fields{
		"package": pkg.Name,
		"version": pkg.K3sVersion,
	})
	sort.Strings(d.pkg.Archs)

	return d, d.validateVersion()
}

type Downloader struct {
	source           string
	sourceURL        string
	basePath         string
	imageListContent []byte
	pkg              common.Package
	logger           logrus.FieldLogger
}

func (d *Downloader) DownloadPackage() (string, error) {
	version, err := versionAndBasePath(d.basePath)
	if err != nil {
		return "", err
	}

	toAddArchs, toDelArchs := version.diff(d.pkg)
	if len(toAddArchs) == 0 &&
		len(toDelArchs) == 0 &&
		isDone(d.basePath) {
		d.logger.Info("the package %s is ready, skip downloading resources.", d.pkg.Name)
		return d.basePath, nil
	}
	if err := d.writeVersion(); err != nil {
		return "", err
	}

	for _, arch := range toDelArchs {
		d.logger.Infof("removing package arch %s", arch)
		if err := os.RemoveAll(filepath.Join(d.basePath, arch)); err != nil {
			return "", err
		}
	}

	for _, arch := range d.pkg.Archs {
		if err := os.MkdirAll(filepath.Join(d.basePath, arch), 0755); err != nil {
			return "", err
		}
		d.logger.Infof("download %s resources", arch)
		if err := d.downloadArch(arch); err != nil {
			return "", err
		}
	}

	if err := done(d.basePath); err != nil {
		return "", err
	}

	_, err = VerifyFiles(d.basePath)

	return d.basePath, err
}

func (d *Downloader) downloadArch(arch string) error {
	if err := d.checkArchExists(arch); err != nil {
		return err
	}
	basePath := filepath.Join(d.basePath, arch)
	if isDone(basePath) {
		d.logger.Infof("arch %s has downloaded, skipped download process.", arch)
		return nil
	}

	archImageList := filepath.Join(basePath, imageListFilename)
	if err := ioutil.WriteFile(archImageList, d.imageListContent, 0644); err != nil {
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
			if err := download(fullPath, d.getFileURL(resourceName)); err != nil {
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

func (d *Downloader) getFileURL(Filename string) string {
	return fmt.Sprintf("%s/%s", d.sourceURL, Filename)
}

// validateVersion will download k3s-images.txt to check the version exists or not.
func (d *Downloader) validateVersion() error {
	downloadURL := d.getFileURL(imageListFilename)
	d.logger.Debugf("downloading images file from %s", downloadURL)
	resp, err := http.Get(downloadURL)
	if err != nil {
		return errors.Wrapf(err, "failed to download image list of k3s version %s, this version may be not validated.", d.pkg.K3sVersion)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		content, _ := ioutil.ReadAll(resp.Body)
		d.logger.Debugf("failed to download image list resource, status code %d, data %s", resp.StatusCode, string(content))
		return ErrVersionNotFound
	}

	content, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	d.imageListContent = content
	return nil
}

// writeVersion will also remove .done file as we assume that creating/updating version is happening.
func (d *Downloader) writeVersion() error {
	versionPath := filepath.Join(d.basePath, versionFilename)
	versionJSON := versionContent(d.pkg)
	_ = os.RemoveAll(versionPath)
	_ = os.RemoveAll(getDonePath(d.basePath))
	d.logger.Info("generating version file")
	return ioutil.WriteFile(versionPath, versionJSON, 0644)
}

// checkArchExists will fire HEAD request to server and check response
func (d *Downloader) checkArchExists(arch string) error {
	target := checksumBaseName + "-" + arch + checksumExt
	resp, err := http.Head(d.getFileURL(target))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if int(resp.StatusCode/100) != 2 {
		return fmt.Errorf("%s may not exist", arch)
	}
	return nil
}

func download(file, fromURL string) error {
	resp, err := http.Get(fromURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if int(resp.StatusCode/100) != 2 {
		content, _ := ioutil.ReadAll(resp.Body)
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
	data, err := ioutil.ReadFile(versionPath)
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
	contents, err := ioutil.ReadDir(basePath)
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

func verifyArchFiles(arch, basePath string) error {
	archBase := filepath.Join(basePath, arch)
	checksumMap, err := getHashMapFromFile(filepath.Join(archBase, checksumFilename))
	if err != nil {
		return errors.Wrapf(err, "failed to get file hash map for arch %s", arch)
	}

	for basename, v := range resourceSuffixes {
		if basename == checksumBaseName {
			continue
		}
		checked := false
		for origin, suffix := range getSuffixMapWithArchs(arch, basename, v) {
			localFileName := basename + origin
			resourceName := basename + suffix

			ok, err := checkFileHash(filepath.Join(archBase, localFileName), checksumMap[resourceName])
			if os.IsNotExist(err) {
				continue
			}
			if !ok {
				return fmt.Errorf("checksum for file %s/%s mismatch", arch, localFileName)
			}
			checked = true
			break
		}
		if !checked {
			return fmt.Errorf("resource %s for %s check fail", basename, arch)
		}
	}
	return nil
}

func VerifyFiles(basePath string) (*common.Package, error) {
	version, err := versionAndBasePath(basePath)
	if err != nil {
		return nil, err
	}
	if version == nil {
		return nil, errors.New("version.json is missing")
	}

	for _, arch := range version.Archs {
		archBase := filepath.Join(basePath, arch)
		if !isDone(archBase) {
			return nil, fmt.Errorf("%s resources aren't available", arch)
		}
		if err := verifyArchFiles(arch, basePath); err != nil {
			return nil, err
		}
	}

	return &common.Package{
		Archs:      version.Archs,
		K3sVersion: version.Version,
	}, nil
}

func getSuffixMapWithArchs(arch, baseName string, suffixes []string) map[string]string {
	rtn := make(map[string]string, len(suffixes))
	for _, suffix := range suffixes {
		if baseName == "k3s" && arch == "amd64" {
			rtn[suffix] = suffix
		} else if baseName == "k3s" && arch == "arm" {
			// arm binary suffix is armhf
			rtn[suffix] = "-armhf" + suffix
		} else {
			rtn[suffix] = "-" + arch + suffix
		}
	}
	return rtn
}

func getExt(filename string) (string, string) {
	name := filename
	var ext, currentExt string

	for currentExt = filepath.Ext(name); currentExt != ""; currentExt = filepath.Ext(name) {
		ext = currentExt + ext
		name = strings.TrimSuffix(name, currentExt)
	}
	return name, ext
}

func getHashMapFromFile(path string) (map[string]string, error) {
	fp, err := os.Open(path)
	if err != nil {
		return nil, errors.Wrapf(err, "checksum file not found")
	}
	defer fp.Close()
	checksumMap := map[string]string{}
	reader := bufio.NewReader(fp)
	for {
		// checksum file should be small enough to ignore isPrefix options.
		line, _, err := reader.ReadLine()
		if err == io.EOF {
			break
		} else if err != nil {
			return nil, err
		}
		arr := separator.Split(string(line), 2)
		checksumMap[filepath.Base(arr[1])] = arr[0]
	}
	return checksumMap, nil
}

func checkFileHash(filepath, targetHash string) (bool, error) {
	hasher := sha256.New()
	fp, err := os.Open(filepath)
	if err != nil {
		return false, err
	}
	defer fp.Close()
	if _, err := io.Copy(hasher, fp); err != nil {
		return false, err
	}
	return fmt.Sprintf("%x", hasher.Sum(nil)) == targetHash, nil
}

func isDone(basePath string) bool {
	done, _ := os.Lstat(getDonePath(basePath))
	return done != nil
}

func done(basePath string) error {
	return ioutil.WriteFile(getDonePath(basePath), []byte{}, 0644)
}

func getDonePath(basePath string) string {
	return filepath.Join(basePath, doneFilename)
}

func RemovePackage(name string) error {
	return os.RemoveAll(PackagePath(name))
}

func TempDir(name string) string {
	return filepath.Join(packageTmpBasePath, name)
}

func PackagePath(name string) string {
	return filepath.Join(packagePath, name)
}
