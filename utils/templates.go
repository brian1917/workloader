package utils

// RootTemplate returns the root usage template
func RootTemplate() string {
	return `  Usage:{{if .Runnable}}
	{{.CommandPath}} [command]

  PCE Management Commands:{{range .Commands}}{{if (or (eq .Name "set-proxy") (eq .Name "clear-proxy") (eq .Name "pce-remove") (eq .Name "pce-add") (eq .Name "get-default") (eq .Name "settings") (eq .Name "pce-list"))}}
	{{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}{{if .HasAvailableLocalFlags}}

  Import/Export Commands:{{range .Commands}}{{if (or (eq .Name "wkld-export") (eq .Name "wkld-import") (eq .Name "ven-export") (eq .Name "ven-import") (eq .Name "ipl-export") (eq .Name "ipl-import") (eq .Name "ipl-replace") (eq .Name "label-export") (eq .Name "label-import") (eq .Name "label-dimension-export") (eq .Name "label-dimension-import") (eq .Name "svc-export") (eq .Name "svc-import") (eq .Name "rule-export") (eq .Name "rule-import") (eq .Name "ruleset-export") (eq .Name "ruleset-import") (eq .Name "eb-export") (eq .Name "eb-import") (eq .Name "labelgroup-export") (eq .Name "labelgroup-import") (eq .Name "cwp-export") (eq .Name "cwp-import") (eq .Name "flow-import"))}}
	{{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}{{if .HasAvailableLocalFlags}}
	  
  Automation Commands:{{range .Commands}}{{if (or (eq .Name "azure-label") (eq .Name "aws-label") (eq .Name "gcp-label") (eq .Name "subnet") (eq .Name "hostparse") (eq .Name "dag-sync"))}}
	{{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}{{if .HasAvailableLocalFlags}}

  Workload Management Commands:{{range .Commands}}{{if (or (eq .Name "compatibility") (eq .Name "mode") (eq .Name "upgrade") (eq .Name "unpair") (eq .Name "get-pk") (eq .Name "umwl-cleanup") (eq .Name "nic-manage") (eq .Name "containment-switch") (eq .Name "increase-ven-rate") (eq .Name "wkld-replicate") (eq .Name "wkld-label"))}}
	{{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}{{if .HasAvailableLocalFlags}}

  Label Management Commands:{{range .Commands}}{{if (or (eq .Name "labels-delete-unused") (eq .Name "label-rename"))}}
	{{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}{{if .HasAvailableLocalFlags}}

  Reporting Commands:{{range .Commands}}{{if (or (eq .Name "rule-usage") (eq .Name "port-usage") (eq .Name "mislabel") (eq .Name "dupecheck") (eq .Name "flowsummary") (eq .Name "traffic") (eq .Name "nic-export") (eq .Name "service-finder") (eq .Name "process-export") (eq .Name "wkld-ipl-mapping") (eq .Name "ven-health") (eq .Name "unused-umwl"))}}
	{{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}{{if .HasAvailableLocalFlags}}

  Multiple PCE Prefix Commands:{{range .Commands}}{{if (or (eq .Name "all-pces") (eq .Name "target-pces"))}}
	{{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}{{if .HasAvailableLocalFlags}}

  Template Commands:{{range .Commands}}{{if (or (eq .Name "template-list") (eq .Name "template-import") (eq .Name "template-create"))}}
	{{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}{{if .HasAvailableLocalFlags}}

  Other Commands:{{range .Commands}}{{if (or (eq .Name "delete") (eq .Name "netscaler-sync"))}}
	{{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}{{if .HasAvailableLocalFlags}}

  Version Command:{{range .Commands}}{{if (or (eq .Name "version") (eq .Name "check-version"))}}
	{{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}{{if .HasAvailableLocalFlags}}
  
Use "{{.CommandPath}} [command] --help" for more information on a command.{{end}}

  `
}

// SubCmdTemplate returns the usage template used for all subcommands
func SubCmdTemplate() string {
	return `
  Usage:{{if .Runnable}}
    {{.UseLine}}{{end}}{{if .HasAvailableSubCommands}}
    {{.CommandPath}} [command]{{end}}{{if gt (len .Aliases) 0}}
  
  Aliases:
    {{.NameAndAliases}}{{end}}{{if .HasExample}}
  
  Examples:
  {{.Example}}{{end}}{{if .HasAvailableSubCommands}}
  
  Available Commands:{{range .Commands}}{{if (or .IsAvailableCommand (eq .Name "help"))}}
    {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}{{if .HasAvailableLocalFlags}}
  
  Flags:
{{.LocalFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasAvailableInheritedFlags}}
  
  Global Flags (not relevant for all commands):
{{.InheritedFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasHelpSubCommands}}
  
  Additional help topics:{{range .Commands}}{{if .IsAdditionalHelpTopicCommand}}
	{{rpad .CommandPath .CommandPathPadding}} {{.Short}}{{end}}{{end}}{{end}}{{if .HasAvailableSubCommands}}
  
  Use "{{.CommandPath}} [command] --help" for more information about a command.{{end}}
  
`
}

// SRootCmdTemplate returns the usage template for sub root commands
func SRootCmdTemplate() string {
	return `
  Usage:{{if .Runnable}}
    {{.CommandPath}} [sub-command]{{end}}{{if gt (len .Aliases) 0}}
  
  Aliases:
    {{.NameAndAliases}}{{end}}{{if .HasExample}}
  
  Examples:
  {{.Example}}{{end}}{{if .HasAvailableSubCommands}}
  
  Available Sub-Commands:{{range .Commands}}{{if (or .IsAvailableCommand (eq .Name "help"))}}
    {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}{{if .HasAvailableLocalFlags}}
  
  Flags:
{{.LocalFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasAvailableInheritedFlags}}
  
  Use "{{.CommandPath}} [command] --help" for more information about a command.{{end}}
  
`
}
