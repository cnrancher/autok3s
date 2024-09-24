package airgap

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/cnrancher/autok3s/pkg/common"
	"github.com/cnrancher/autok3s/pkg/hosts"
	"github.com/cnrancher/autok3s/pkg/hosts/dialer"
	"github.com/cnrancher/autok3s/pkg/settings"
	"github.com/cnrancher/autok3s/pkg/types"

	"github.com/pkg/errors"
	"github.com/pkg/sftp"
	"github.com/sirupsen/logrus"
)

type fileMap struct {
	mode           os.FileMode
	dataDirSubpath string
	targetPath     string
}

const (
	installScriptName       = "install.sh"
	defaultDataDirPath      = "/var/lib/rancher/k3s"
	dataDirParamPrefix      = "--data-dir"
	dataDirParamPrefixShort = "-d"
)

var (
	errArchNotSupport = errors.New("arch not support")
	unameCommand      = "uname -m"
	remoteTmpDir      = "/tmp/autok3s"
	// mapping file kind to real filename in remote host
	remoteFileMap = map[string]fileMap{
		"k3s": {
			mode:       0755,
			targetPath: "/usr/local/bin",
		},
		"k3s-airgap-images": {
			mode:           0644,
			dataDirSubpath: "agent/images",
		},
		installScriptName: {
			mode:       0755,
			targetPath: "/usr/local/bin",
		},
	}
	parseArchMap = map[string]string{
		"x86_64":  "amd64",
		"aarch64": "arm64",
		"armv7l":  "arm",
	}
)

func ScpFiles(logger *logrus.Logger, clusterName string, pkg *common.Package, dialer *dialer.SSHDialer, extraArgs string) (er error) {
	dataPath := getDataPath(extraArgs)
	conn := dialer.GetClient()
	fieldLogger := logger.WithFields(logrus.Fields{
		"cluster":   clusterName,
		"component": "airgap",
	})
	installScript := settings.InstallScript.Get()
	if installScript == "" {
		return errors.New("install script must be configured")
	}

	arch, err := getRemoteArch(dialer)
	if err != nil {
		return err
	}
	if ValidatedArch[arch] {
		return errors.Wrapf(errArchNotSupport, "remote server arch: %s", arch)
	}
	if !pkg.Archs.Contains(arch) {
		return fmt.Errorf("%s resource doesn't exist in package %s", arch, packagePath)
	}

	fieldLogger.Infof("Get remote server arch %s", arch)
	files, err := getScpFileMap(arch, pkg)
	if err != nil {
		return err
	}

	scpClient, err := sftp.NewClient(conn)
	if err != nil {
		return err
	}
	defer scpClient.Close()

	fieldLogger.Infof("connected to remote server %s with sftp", conn.RemoteAddr())

	//scp files to tmp dir
	tmpDir := getRemoteTmpDir(clusterName)
	if err := scpClient.MkdirAll(tmpDir); err != nil {
		return err
	}
	defer func() { _ = scpClient.RemoveDirectory(tmpDir) }()

	// scp files and execute post scp commands
	for local, remote := range files {
		filename := filepath.Base(local)
		remoteFileName := filepath.Join(tmpDir, filename)
		var source io.Reader
		if local == installScriptName {
			source = bytes.NewBufferString(installScript)
		} else {
			fp, err := os.Open(local)
			if err != nil {
				return err
			}
			defer fp.Close()
			source = fp
		}
		rfp, err := scpClient.Create(remoteFileName)
		if err != nil {
			return err
		}
		defer rfp.Close()
		fieldLogger.Infof("local file %s", local)
		fieldLogger.Infof("copy to remote %s", remoteFileName)
		if _, err := io.Copy(rfp, source); err != nil {
			return err
		}
		fieldLogger.Infof("setting file %s mode, %s", filename, remote.mode)
		if err := scpClient.Chmod(remoteFileName, remote.mode); err != nil {
			return err
		}

		targetPath := remote.targetPath
		if remote.dataDirSubpath != "" {
			targetPath = filepath.Join(dataPath, remote.dataDirSubpath)
		}
		targetFilename := filepath.Join(targetPath, filename)
		moveCMD := fmt.Sprintf("mkdir -p %s;mv %s %s", targetPath, remoteFileName, targetFilename)
		fieldLogger.Infof("executing cmd in remote server %s", moveCMD)
		if output, err := dialer.ExecuteCommands(moveCMD); err != nil {
			fieldLogger.Errorf("failed to execute cmd %s, output: %s, %v", moveCMD, output, err)
			return err
		}

		fieldLogger.Infof("file moved to %s", targetFilename)
		fieldLogger.Infof("remote file %s transferred", filename)

		defer func(tmpFilename, targetFilename string) {
			//clean up process when return error
			if er != nil {
				fieldLogger.Warnf("error occurs when transferring resources, following resources should be clean later: %s %s", tmpFilename, targetFilename)
			}
		}(remoteFileName, targetFilename)
	}

	fieldLogger.Info("all files transferred")
	return nil
}

func PreparePackage(cluster *types.Cluster) (*common.Package, error) {
	clusterName := cluster.Name
	packageName := cluster.PackageName
	packagePath := cluster.PackagePath

	if packageName == "" && packagePath == "" {
		return nil, nil
	}

	if packagePath == "" && packageName != "" {
		pkgs, err := common.DefaultDB.ListPackages(&packageName)
		if err != nil {
			return nil, err
		}
		return &pkgs[0], nil
	}

	fieldLogger := logrus.WithFields(logrus.Fields{
		"cluster":   clusterName,
		"component": "airgap",
	})
	info, err := os.Lstat(packagePath)
	if err != nil {
		return nil, err
	}
	var tmpPath, currentPath string
	if !info.IsDir() {
		tmpPath, err = SaveToTmp(packagePath, "cluster-"+clusterName)
		if err != nil {
			_ = os.RemoveAll(tmpPath)
			return nil, err
		}
		fieldLogger.Infof("created tmp directory %s for package %s, will be removed after", tmpPath, packagePath)
		currentPath = tmpPath
	} else {
		currentPath = packagePath
	}

	rtn, err := VerifyFiles(currentPath)
	if err != nil {
		if tmpPath != "" {
			_ = os.RemoveAll(currentPath)
		}
		return nil, err
	}
	rtn.FilePath = currentPath
	fieldLogger.Infof("airgap package %s validated", packagePath)
	return rtn, nil
}

func getRemoteArch(executor hosts.Script) (string, error) {
	line, err := executor.ExecuteCommands(unameCommand)
	if err != nil {
		return "", err
	}
	line = strings.TrimSuffix(line, "\n")
	return parseUnameArch(string(line)), nil
}

func parseUnameArch(output string) string {
	rtn, ok := parseArchMap[output]
	if ok {
		return rtn
	}
	return output
}

// getScpFileMap will return the local file path for specific arch to target file map
func getScpFileMap(arch string, pkg *common.Package) (map[string]fileMap, error) {
	var rtn = make(map[string]fileMap, len(remoteFileMap))
	archBasePath := filepath.Join(pkg.FilePath, arch)
	for key, file := range remoteFileMap {
		if key == installScriptName {
			rtn[installScriptName] = file
			continue
		}
		hasFile := false
		for _, suffix := range resourceSuffixes[key] {
			filename := filepath.Join(archBasePath, key+suffix)
			if _, err := os.Lstat(filename); err != nil {
				continue
			}
			hasFile = true
			rtn[filename] = file
		}
		if !hasFile {
			return nil, fmt.Errorf("resource file %s is missing in package %s", key, pkg.FilePath)
		}
	}
	return rtn, nil
}

func getRemoteTmpDir(clustername string) string {
	return filepath.Join(remoteTmpDir, clustername)
}

func getDataPath(extraArgs string) string {
	dataPath := defaultDataDirPath
	args := strings.Split(extraArgs, " ")
	for i, arg := range args {
		var prefix string
		if strings.HasPrefix(arg, dataDirParamPrefix) {
			prefix = dataDirParamPrefix
		}
		if strings.HasPrefix(arg, dataDirParamPrefixShort) {
			prefix = dataDirParamPrefixShort
		}
		if prefix == "" {
			continue
		}
		// if the arg == dataPath prefix, return the next arg
		if len(arg) == len(prefix) && i < len(args)-1 {
			return args[i+1]
		}
		// this is the case as -d=/data/xxx
		if len(arg) > len(prefix) && arg[len(prefix)] == '=' {
			return strings.TrimPrefix(arg, prefix+"=")
		}
		// only two cases above are validated, otherwise return the default path
		if prefix != "" {
			break
		}
	}
	return dataPath
}
