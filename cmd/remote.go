package cmd

import (
	"fmt"
	"strings"
	"sync"

	"github.com/spf13/pflag"
)

var remoteValidationMu sync.Mutex

// ValidateRemoteInvocation permits service operations, not local credential
// management, shell access or reads of files on the controlling machine.
func ValidateRemoteInvocation(args []string) error {
	remoteValidationMu.Lock()
	defer remoteValidationMu.Unlock()
	root := NewRootCmd("mw")
	command, remaining, err := root.Find(args)
	if err != nil {
		return err
	}
	allowed := map[string]bool{
		"mw phabricator view": true, "mw phabricator read": true, "mw phabricator comments": true,
		"mw phabricator create":    true,
		"mw phabricator task list": true, "mw phabricator task view": true,
		"mw phabricator task create": true, "mw phabricator task edit": true,
		"mw phabricator project list": true, "mw phabricator project view": true,
		"mw phabricator board columns": true,
		"mw phabricator paste view":    true, "mw phabricator paste create": true, "mw phabricator paste edit": true,
		"mw phabricator auth status": true, "mw phabricator auth profiles": true,
	}
	if !allowed[command.CommandPath()] {
		return fmt.Errorf("%s is not available through the session bridge", command.CommandPath())
	}
	if err := command.ParseFlags(remaining); err != nil {
		return err
	}
	if err := command.ValidateArgs(command.Flags().Args()); err != nil {
		return err
	}
	command.Flags().Visit(func(flag *pflag.Flag) {
		switch flag.Name {
		case "file", "description-file", "input":
			if flag.Value.String() != "-" {
				err = fmt.Errorf("%s must use stdin through the bridge", flag.Name)
			}
		case "output":
			if strings.EqualFold(flag.Value.String(), "web") {
				err = fmt.Errorf("open web links from the remote terminal instead of delegating browser launches")
			}
		}
	})
	return err
}
