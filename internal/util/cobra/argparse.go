package cobrautil

import "regexp"

/*
CommandAndEnvFromArgs takes arguments passed to a cobra command and extracts any prefixing env var definitions from them.

For example, an argument of "FOO=bar echo foo" will return ["echo", "foo"] and ["FOO=bar"].
*/
func CommandAndEnvFromArgs(in []string) (args []string, envs []string) {
	args = []string{}
	envs = []string{}
	regex, _ := regexp.Compile(`^\w+=\w+$`)
	for _, arg := range in {
		matched := regex.MatchString(arg)
		if matched {
			envs = append(envs, arg)
		} else {
			args = append(args, arg)
		}
	}
	return args, envs
}
