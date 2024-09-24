package airgap

import (
	"bufio"
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/cnrancher/autok3s/pkg/common"
	"github.com/cnrancher/autok3s/pkg/settings"
	"github.com/pkg/errors"
)

var (
	packagePath        = filepath.Join(common.CfgPath, "package")
	packageTmpBasePath = filepath.Join(packagePath, tmpDirName)
	downloadSourceMap  = map[string]string{
		"github":    "https://github.com/k3s-io/k3s/releases/download",
		"aliyunoss": "https://rancher-mirror.rancher.cn/k3s",
	}
	client = http.Client{
		Timeout: 45 * time.Second,
	}
)

func getSourceURL(version string) string {
	source := settings.PackageDownloadSource.Get()
	baseURL := downloadSourceMap[source]
	if baseURL == "" {
		source = "github"
		baseURL = downloadSourceMap[source]
	}

	versionPath := version
	if source == "aliyunoss" {
		versionPath = strings.ReplaceAll(versionPath, "+", "-")
	}
	versionPath = url.QueryEscape(versionPath)
	return fmt.Sprintf("%s/%s", baseURL, versionPath)
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

func doRequestWithCtx(ctx context.Context, method, url string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, err
	}
	return client.Do(req)
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
	return os.WriteFile(getDonePath(basePath), []byte{}, 0644)
}

func getDonePath(basePath string) string {
	return filepath.Join(basePath, doneFilename)
}

func updatePackageState(pkg *common.Package, state common.State) error {
	pkg.State = state
	return common.DefaultDB.SavePackage(*pkg)
}

func GetDownloadFilePath(name string) string {
	return filepath.Join(PackagePath(name), "log")
}

func GetLogFile(name string) (logFile *os.File, err error) {
	logFilePath := GetDownloadFilePath(name)
	if err = os.MkdirAll(filepath.Dir(logFilePath), 0755); err != nil {
		return nil, err
	}
	// check file exist
	_, err = os.Stat(logFilePath)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}
		logFile, err = os.Create(logFilePath)
	} else {
		logFile, err = os.OpenFile(logFilePath, os.O_APPEND|os.O_WRONLY, os.ModeAppend)
	}
	return logFile, err
}
