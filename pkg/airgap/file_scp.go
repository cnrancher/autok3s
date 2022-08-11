package airgap

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	"github.com/cnrancher/autok3s/pkg/common"
	"github.com/cnrancher/autok3s/pkg/hosts"
	"github.com/cnrancher/autok3s/pkg/settings"
	"github.com/cnrancher/autok3s/pkg/types"

	"github.com/pkg/errors"
	"github.com/pkg/sftp"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"
)

type fileMap struct {
	mode       os.FileMode
	targetPath string
}

const (
	installScriptName = "install.sh"
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
			mode:       0644,
			targetPath: "/var/lib/rancher/k3s/agent/images",
		},
		installScriptName: {
			mode:       0755,
			targetPath: "/usr/local/bin",
		},
	}
)

func ScpFiles(clusterName string, pkg *common.Package, dialer *hosts.SSHDialer) error {
	conn := dialer.GetClient()
	fieldLogger := logrus.WithFields(logrus.Fields{
		"cluster":   clusterName,
		"component": "airgap",
	})
	installScript := settings.InstallScript.Get()
	if installScript == "" {
		return errors.New("install script must be configured")
	}

	arch, err := getRemoteArch(conn)
	if err != nil {
		return err
	}
	if ok, _ := ValidatedArch[arch]; !ok {
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
	defer scpClient.RemoveDirectory(tmpDir)

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

		targetFilename := filepath.Join(remote.targetPath, filename)
		var stdout, stderr bytes.Buffer
		dialer = dialer.SetStdio(&stdout, &stderr, nil)
		moveCMD := fmt.Sprintf("sudo mkdir -p %s;sudo mv %s %s", remote.targetPath, remoteFileName, targetFilename)
		fieldLogger.Infof("executing cmd in remote server %s", moveCMD)
		if err := dialer.Cmd(moveCMD).Run(); err != nil {
			fieldLogger.Errorf("failed to execute cmd %s, stdout: %s, stderr: %s, %v", moveCMD, stdout.String(), stderr.String(), err)
			return err
		}
		fieldLogger.Infof("file moved to %s", targetFilename)
		fieldLogger.Infof("remote file %s transferred", filename)
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

func getRemoteArch(conn *ssh.Client) (string, error) {
	session, err := conn.NewSession()
	if err != nil {
		return "", err
	}
	defer session.Close()

	stdoutPipe, err := session.StdoutPipe()
	if err != nil {
		return "", err
	}
	stderrPipe, err := session.StderrPipe()
	if err != nil {
		return "", err
	}

	outWriter := bytes.NewBuffer([]byte{})
	errWriter := bytes.NewBuffer([]byte{})

	wg := sync.WaitGroup{}

	wg.Add(1)
	go func() {
		_, _ = io.Copy(outWriter, stdoutPipe)
		wg.Done()
	}()

	wg.Add(1)
	go func() {
		_, _ = io.Copy(errWriter, stderrPipe)
		wg.Done()
	}()

	err = session.Run(unameCommand)

	wg.Wait()
	if err != nil {
		return "", err
	}
	remoteErr := errWriter.String()
	if remoteErr != "" {
		return "", fmt.Errorf("got errors from remote server, %s", remoteErr)
	}
	bufferReader := bufio.NewReader(outWriter)
	line, _, err := bufferReader.ReadLine()
	if err != nil && err != io.EOF {
		return "", err
	} else if err == io.EOF {
		line = []byte{}
	}
	return parseUnameArch(string(line)), nil
}

func parseUnameArch(output string) string {
	switch output {
	case "x86_64":
		return "amd64"
	case "aarh64":
		return "arm64"
	case "armv7l":
		return "arm"
	default:
		return output
	}
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
