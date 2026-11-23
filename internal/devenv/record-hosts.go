package devenv

import (
	"os"

	"gitlab.wikimedia.org/repos/releng/cli/internal/util/files"
)

func (m Environment) hostRecordFile() string {
	return m.Directory() + string(os.PathSeparator) + "record-hosts"
}

/*RecordHostUsageBySite records a host in a local file as used at some point.*/
func (m Environment) RecordHostUsageBySite(host string) error {
	return files.AddLineUnique(host, m.hostRecordFile())
}

func (m Environment) RemoveUsedHostsIfExists() error {
	return files.RemoveIfExists(m.hostRecordFile())
}

/*UsedHosts lists all hosts that have been used at some point.*/
func (m Environment) UsedHosts() ([]string, error) {
	lines, err := files.Lines(m.hostRecordFile())
	if os.IsNotExist(err) {
		return []string{}, nil
	}
	return lines, err
}
