package mediawiki

import (
	"fmt"
	"os"
	osexec "os/exec"

	"gitlab.wikimedia.org/repos/releng/cli/internal/exec"
	stringsutil "gitlab.wikimedia.org/repos/releng/cli/internal/util/strings"
)

func exitIfNoGit() {
	_, err := osexec.LookPath("git")
	if err != nil {
		fmt.Println("You must have git installed on your system.")
		os.Exit(1)
	}
}

/*CloneOpts for use with GithubCloneMediaWiki.*/
type CloneOpts = struct {
	GetMediaWiki          bool
	GetVector             bool
	GetGerritSkins        []string
	GetGerritExtensions   []string
	UseGithub             bool
	UseShallow            bool
	GerritInteractionType string
	GerritUsername        string
}

/*CloneSetup provides a packages initial setup method for MediaWiki etc with some speedy features.*/
func (m MediaWiki) CloneSetup(options CloneOpts) {
	exitIfNoGit()

	fmt.Println("Cloning repositories...")

	// options.GetVector is deprecated, so shift its info into options.GetGerritSkins
	if options.GetVector && !stringsutil.StringInSlice("Vector", options.GetGerritSkins) {
		options.GetGerritSkins = append(options.GetGerritSkins, "Vector")
	}

	if options.GetMediaWiki {
		startRemoteCore := "https://gerrit.wikimedia.org/r/mediawiki/core"
		if options.UseGithub {
			startRemoteCore = "https://github.com/wikimedia/mediawiki.git"
		}

		endRemoteCore := ""
		if options.GerritInteractionType == "http" {
			endRemoteCore = "https://gerrit.wikimedia.org/r/mediawiki/core"
		} else if options.GerritInteractionType == "ssh" {
			endRemoteCore = "ssh://" + options.GerritUsername + "@gerrit.wikimedia.org:29418/mediawiki/core"
		} else {
			fmt.Println("Unknown GerritInteractionType")
			os.Exit(1)
		}
		cloneAndSetRemote(m.Path(""), startRemoteCore, endRemoteCore, options.UseShallow)
	}

	for _, skinName := range options.GetGerritSkins {
		startRemote := gerritHTTPRemoteForSkin(skinName)
		if options.UseGithub {
			startRemote = githubRemoteForSkin(skinName)
		}

		endRemote := ""
		if options.GerritInteractionType == "http" {
			endRemote = gerritHTTPRemoteForSkin(skinName)
		} else if options.GerritInteractionType == "ssh" {
			endRemote = gerritSSHRemoteForSkin(skinName, options.GerritUsername)
		} else {
			fmt.Println("Unknown GerritInteractionType")
			os.Exit(1)
		}

		cloneAndSetRemote(m.Path("skins/"+skinName), startRemote, endRemote, options.UseShallow)
	}

	for _, extensionName := range options.GetGerritExtensions {
		startRemote := gerritHTTPRemoteForExtension(extensionName)
		if options.UseGithub {
			startRemote = githubRemoteForExtension(extensionName)
		}

		endRemote := ""
		if options.GerritInteractionType == "http" {
			endRemote = gerritHTTPRemoteForExtension(extensionName)
		} else if options.GerritInteractionType == "ssh" {
			endRemote = gerritSSHRemoteForExtension(extensionName, options.GerritUsername)
		} else {
			fmt.Println("Unknown GerritInteractionType")
			os.Exit(1)
		}

		cloneAndSetRemote(m.Path("extensions/"+extensionName), startRemote, endRemote, options.UseShallow)
	}

	fmt.Println("Repositories cloned.")
}

func gerritHTTPRemoteForSkin(skin string) string {
	return "https://gerrit.wikimedia.org/r/mediawiki/skins/" + skin
}

func gerritSSHRemoteForSkin(skin string, username string) string {
	return "ssh://" + username + "@gerrit.wikimedia.org:29418/mediawiki/skins/" + skin
}

func githubRemoteForSkin(skin string) string {
	return "https://github.com/wikimedia/mediawiki-skins-" + skin + ".git"
}

func gerritHTTPRemoteForExtension(extension string) string {
	return "https://gerrit.wikimedia.org/r/mediawiki/extensions/" + extension
}

func gerritSSHRemoteForExtension(extension string, username string) string {
	return "ssh://" + username + "@gerrit.wikimedia.org:29418/mediawiki/skins/" + extension
}

func githubRemoteForExtension(extension string) string {
	return "https://github.com/wikimedia/mediawiki-extensions-" + extension + ".git"
}

func cloneAndSetRemote(directory string, startRemote string, endRemote string, useShallow bool) {
	exec.RunTTYCommand(exec.Command(
		"git",
		gitCloneArguments(directory, startRemote, useShallow)...,
	))
	if startRemote != endRemote {
		exec.RunTTYCommand(exec.Command(
			"git",
			gitRemoteSetURLArguments(directory, endRemote)...,
		))
	}
}

func gitCloneArguments(directory string, remote string, useShallow bool) []string {
	args := []string{"clone"}
	if useShallow {
		args = append(args, "--depth=1")
	}
	args = append(args, remote)
	args = append(args, directory)
	return args
}

func gitRemoteSetURLArguments(directory string, newRemote string) []string {
	return []string{"-C", directory, "remote", "set-url", "origin", newRemote}
}
