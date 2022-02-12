  Usage:
  {{- if .Runnable}}{{print "\n"}}{{.UseLine | indent 4}}{{end}}
  {{- if .HasAvailableSubCommands}}{{print "\n"}}{{.CommandPath | indent 4}} [command]{{end}}

{{- if gt (len .Aliases) 0}}{{print "\n"}}
  Aliases:
    {{.NameAndAliases}}
{{- end}}

{{- if .HasExample}}{{print "\n"}}
  Examples:
{{.Example | indent 4 }}
{{- end}}

{{- if .HasAvailableSubCommands}}{{print "\n"}}  
  Available Commands:{{range .Commands}}{{if (or .IsAvailableCommand (eq .Name "help"))}}
{{rpad .Name .NamePadding | indent 4 }} {{.Short}}{{end}}{{end}}
{{- end}}

{{- if .HasAvailableLocalFlags}}{{print "\n"}}
  Flags:
{{- range splitList "\n" ( .LocalFlags.FlagUsages | trim ) }}
{{. | trim | indent 4}}
{{- end -}}
{{- end}}

{{- if .HasAvailableInheritedFlags}}{{print "\n"}}
  Global Flags:
{{- range splitList "\n" ( .InheritedFlags.FlagUsages | trim ) }}
{{. | trim | indent 4}}
{{- end -}}
{{- end}}

{{- if .HasHelpSubCommands}}{{print "\n"}}
  Additional help topics:{{range .Commands}}

  {{- if .IsAdditionalHelpTopicCommand}}
    {{rpad .CommandPath .CommandPathPadding}} {{.Short}}{{end}}
  {{- end}}
{{- end}}
    
{{- if .HasAvailableSubCommands}}{{print "\n"}}
  Use "{{.CommandPath}} [command] --help" for more information about a command.
{{- end }}
