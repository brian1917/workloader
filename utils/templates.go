package utils

// RootTemplate returns the root usage template
func RootTemplate() string {
	return `  Usage:{{if .Runnable}}
	{{.CommandPath}} [command]

  Login Commands:{{range .Commands}}{{if (or (eq .Name "login") (eq .Name "logout"))}}
	{{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}{{if .HasAvailableLocalFlags}}

  Import/Export Commands:{{range .Commands}}{{if (or (eq .Name "import") (eq .Name "export") (eq .Name "flowupload"))}}
	{{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}{{if .HasAvailableLocalFlags}}
	  
  Automated Labeling Commands:{{range .Commands}}{{if (or (eq .Name "traffic") (eq .Name "subnet") (eq .Name "hostparse"))}}
	{{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}{{if .HasAvailableLocalFlags}}

  Workload Management Commands:{{range .Commands}}{{if (or (eq .Name "compatibility") (eq .Name "mode") (eq .Name "upgrade"))}}
	{{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}{{if .HasAvailableLocalFlags}}

  Reporting Commands:{{range .Commands}}{{if (or (eq .Name "mislabel") (eq .Name "dupecheck") (eq .Name "flowsummary") (eq .Name "explorer") (eq .Name "nic"))}}
	{{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}{{if .HasAvailableLocalFlags}}

  Version Command:{{range .Commands}}{{if (or (eq .Name "version"))}}
	{{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}{{if .HasAvailableLocalFlags}}
  
Use "{{.CommandPath}} [command] --help" for more information about a command.{{end}}

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
  
  Global Flags:
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
