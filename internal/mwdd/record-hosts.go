package mwdd

import (
	"os"

	"gitlab.wikimedia.org/repos/releng/cli/internal/util/files"
)

func (m MWDD) hostRecordFile() string {
	return m.Directory() + string(os.PathSeparator) + "record-hosts"
}

/*RecordHostUsageBySite records a host in a local file as used at some point.*/
func (m MWDD) RecordHostUsageBySite(host string) {
	files.AddLineUnique(host, m.hostRecordFile())
}

/*UsedHosts lists all hosts that have been used at some point.*/
func (m MWDD) UsedHosts() []string {
	return files.Lines(m.hostRecordFile())
}
