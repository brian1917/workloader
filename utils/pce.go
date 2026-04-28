package utils

import (
	"fmt"
	"os"
	"strconv"

	"github.com/brian1917/illumioapi"
	"github.com/spf13/viper"
)

// GetTargetPCE gets the target PCE for a command
func GetTargetPCE(GetLabelMaps bool) (illumioapi.PCE, error) {

	// Get the PCE name
	var name string
	if viper.GetString("target_pce") != "" {
		name = viper.GetString("target_pce")
	} else if viper.GetString("default_pce_name") != "" {
		name = viper.GetString("default_pce_name")
	} else {
		LogError("there is no pce set using the --pce flag and there is no default pce. either run workloader pce-add to add your first pce or workloader set-default to set an existing PCE as default.")
	}

	// Get the PCE
	pce, err := GetPCEbyName(name, GetLabelMaps)
	if err != nil {
		return illumioapi.PCE{}, err
	}

	// Adjust PCE for when no auth

	if pce.User == "" {
		if os.Getenv("WORKLOADER_API_USER") == "" {
			return pce, fmt.Errorf("%s does not have an api user and the WORKLOADER_API_USER env variable is not set", name)
		}
		pce.User = os.Getenv("WORKLOADER_API_USER")
	}

	if pce.Key == "" {
		if os.Getenv("WORKLOADER_API_KEY") == "" {
			return pce, fmt.Errorf("%s does not have an api key and the WORKLOADER_API_KEY env variable is not set", name)
		}
		pce.Key = os.Getenv("WORKLOADER_API_KEY")
	}

	if pce.Org == 0 {
		if os.Getenv("WORKLOADER_ORG") == "" {
			return pce, fmt.Errorf("%s does not have an org and the WORKLOADER_ORG env variable is not set", name)
		}
		pce.Org, err = strconv.Atoi(os.Getenv("WORKLOADER_ORG"))
		if err != nil {
			return pce, fmt.Errorf("%s is not valid org for WORKLOADER_ORG env variable", os.Getenv("WORKLOADER_ORG"))
		}
	}

	return pce, nil
}

// GetPCEbyName gets a PCE by it's provided name
func GetPCEbyName(name string, GetLabelMaps bool) (illumioapi.PCE, error) {
	var pce illumioapi.PCE
	if viper.IsSet(name + ".fqdn") {
		pce = illumioapi.PCE{
			FriendlyName:       name,
			FQDN:               viper.GetString(name + ".fqdn"),
			Port:               viper.GetInt(name + ".port"),
			Org:                viper.GetInt(name + ".org"),
			User:               viper.GetString(name + ".user"),
			Key:                viper.GetString(name + ".key"),
			DisableTLSChecking: viper.GetBool(name + ".disableTLSChecking"),
		}
		if viper.GetString(name+".proxy") != "" {
			pce.Proxy = viper.GetString(name + ".proxy")
		}
		if GetLabelMaps {
			apiResps, err := pce.Load(illumioapi.LoadInput{Labels: true})
			LogMultiAPIResp(apiResps)
			if err != nil {
				LogError(err.Error())
			}
		}
		return pce, nil
	}

	return illumioapi.PCE{}, fmt.Errorf("could not retrieve %s PCE information", name)
}

// GetPCEbyName gets a PCE by it's provided name
func GetPCENoAPI(name string) (illumioapi.PCE, error) {
	var pce illumioapi.PCE
	if viper.IsSet(name + ".fqdn") {
		pce = illumioapi.PCE{
			FriendlyName:       name,
			FQDN:               viper.GetString(name + ".fqdn"),
			Port:               viper.GetInt(name + ".port"),
			Org:                viper.GetInt(name + ".org"),
			User:               viper.GetString(name + ".user"),
			Key:                viper.GetString(name + ".key"),
			DisableTLSChecking: viper.GetBool(name + ".disableTLSChecking"),
		}
		if viper.GetString(name+".proxy") != "" {
			pce.Proxy = viper.GetString(name + ".proxy")
		}
		return pce, nil
	}

	return illumioapi.PCE{}, fmt.Errorf("could not retrieve %s PCE information", name)
}

func UseMulti() bool {
	if !viper.IsSet("get_api_behavior") || viper.GetString("get_api_behavior") == "multi" {
		LogInfo("using multi get api behavior", false)
		return true
	}
	LogInfo("using single get api behavior", false)
	return false
}
