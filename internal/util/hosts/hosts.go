package hosts

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/txn2/txeh"
)

var (
	hostsFile      = ""
	hostsTmpPrefix = "mwcli-hosts-"
)

/*ChangeResult result of changing the hosts file.*/
type ChangeResult struct {
	Success   bool
	Altered   bool
	Content   string
	WriteFile string
}

func LocalIP() string {
	_, inGitlabCi := os.LookupEnv("GITLAB_CI")
	if inGitlabCi {
		// Localhost does not refer to our services in Gitlab CI, docker does
		// https://gitlab.com/gitlab-org/gitlab/-/issues/34814#note_426362459
		return IPv4("docker")
	}
	return "127.0.0.1"
}

/*AddHosts attempts to add requested hosts to the system hosts file, and gives you the new content, a tmp file and success.*/
func AddHosts(ip string, toAdd []string, tryWrite bool) ChangeResult {
	hosts := hosts()

	logrus.Tracef("Adding hosts: %v", toAdd)
	hosts.AddHosts(ip, toAdd)
	// TODO when the library supports it do ipv6 too https://github.com/txn2/txeh/issues/15
	// hosts.AddHosts("::1", toAdd)

	return finishChanges(tryWrite, hosts)
}

/*RemoveHostsWithSuffix attempts to remove all hosts with a suffix from the system hosts file, and gives you the new content, a tmp file and success.*/
func RemoveHostsWithSuffix(ip string, hostSuffix string, tryWrite bool) ChangeResult {
	hosts := hosts()
	removeHostsWithSuffixFromLines(hostSuffix, hosts)
	return finishChanges(tryWrite, hosts)
}

/*Writable is the hosts file writable.*/
func Writable() bool {
	return fileIsWritable(FilePath())
}

// FilePath returns the path to the hosts file.
func FilePath() string {
	return hosts().HostsConfig.WriteFilePath
}

func fileIsWritable(filePath string) bool {
	file, err := os.OpenFile(filepath.Clean(filePath), os.O_WRONLY, 0o666) // #nosec G302
	if err != nil {
		return false
	}
	closeErr := file.Close()
	if closeErr != nil {
		panic(closeErr)
	}
	return true
}

func tmpFile() string {
	tmpFile, err := os.CreateTemp(os.TempDir(), hostsTmpPrefix)
	if err != nil {
		panic(err)
	}
	closeErr := tmpFile.Close()
	if closeErr != nil {
		panic(closeErr)
	}
	return tmpFile.Name()
}

func hosts() *txeh.Hosts {
	hosts, err := txeh.NewHosts(&txeh.HostsConfig{
		ReadFilePath:  hostsFile,
		WriteFilePath: hostsFile,
	})
	if err != nil {
		panic(err)
	}
	return hosts
}

func removeHostsWithSuffixFromLines(hostSuffix string, hosts *txeh.Hosts) *txeh.Hosts {
	lines := hosts.GetHostFileLines()

	for _, line := range *lines {
		for _, hostname := range line.Hostnames {
			if strings.HasSuffix(hostname, hostSuffix) {
				hosts.RemoveHost(hostname)
			}
		}
	}

	return hosts
}

func finishChanges(tryWrite bool, touchedHosts *txeh.Hosts) ChangeResult {
	diskHosts := hosts()
	if tryWrite && diskHosts.RenderHostsFile() != touchedHosts.RenderHostsFile() {
		return save(touchedHosts)
	}
	return ChangeResult{
		Success:   true,
		Altered:   false,
		Content:   diskHosts.RenderHostsFile(),
		WriteFile: diskHosts.WriteFilePath,
	}
}

func save(hosts *txeh.Hosts) ChangeResult {
	err := hosts.Save()
	if err != nil {
		tmpFile := tmpFile()
		err = hosts.SaveAs(tmpFile)
		if err != nil {
			panic(err)
		}
		return ChangeResult{
			Success:   false,
			Altered:   true,
			Content:   hosts.RenderHostsFile(),
			WriteFile: tmpFile,
		}
	}

	return ChangeResult{
		Success:   true,
		Altered:   true,
		Content:   hosts.RenderHostsFile(),
		WriteFile: hosts.WriteFilePath,
	}
}
