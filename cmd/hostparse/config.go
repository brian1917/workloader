package hostparse

import (
	"fmt"
	"time"

	"github.com/brian1917/workloader/utils"
)

type config struct {
	Illumio illumio `toml:"illumio"`
	Parser  parser  `toml:"parser"`
	Match   match   `toml:"match"`
	Logging logging `toml:"logging"`
}

type illumio struct {
	NoPCE bool `toml:"no_pce"`
}

type parser struct {
	Parserfile   string `toml:"parserfile"`
	HostnameFile string `toml:"hostnamefile"`
	OutputFile   string `toml:"outputfile"`
	NoPrompt     bool   `toml:"noprompt"`
	CheckCase    int    `toml:"checkcase"`
	Name         bool   `toml:"name"`
}
type match struct {
	AllEmpty    bool   `toml:"allempty"`
	IgnoreMatch bool   `toml:"ignorematch"`
	App         string `toml:"app"`
	Env         string `toml:"env"`
	Loc         string `toml:"loc"`
	Role        string `toml:"role"`
}
type logging struct {
	LogOnly      bool   `toml:"log_only"`
	LogDirectory string `toml:"log_directory"`
	LogFile      string `toml:"log_file"`
	debug        bool
}

func parseConfig() config {

	config := config{
		Illumio: illumio{
			NoPCE: noPCE},
		Parser: parser{
			Parserfile:   parserFile,
			HostnameFile: hostFile,
			OutputFile:   "hostname-parser-output-" + time.Now().Format("20060102_150405") + ".csv",
			NoPrompt:     noPrompt,
			CheckCase:    updatecase,
			Name:         name},
		Match: match{
			AllEmpty:    allEmpty,
			IgnoreMatch: ignoreMatch,
			App:         appFlag,
			Env:         envFlag,
			Loc:         locFlag,
			Role:        roleFlag},
		Logging: logging{
			LogOnly:      logonly,
			LogDirectory: "",
			LogFile:      "workloader-hostname-log-" + time.Now().Format("20060102_150405") + ".csv",
			debug:        debugLogging}}

	if config.Illumio.NoPCE && config.Parser.HostnameFile == "" {
		fmt.Println("You must use the hostfile option when not using PCE Data(no_pce=true)")
		utils.Logger.Fatalf("[ERROR] - hostparser - must use the hostfile option when not using PCE Data(no_pce=true)")
	}

	return config
}
