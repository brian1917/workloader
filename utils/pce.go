package utils

import (
	"fmt"

	"github.com/brian1917/illumioapi"
	"github.com/spf13/viper"
)

// GetTargetPCE gets the target PCE for a command
func GetTargetPCE(GetLabelMaps bool) (illumioapi.PCE, error) {

	// Get the PCE name
	var name string
	if viper.Get("target_pce") != nil && viper.Get("target_pce").(string) !="" {
		name = viper.Get("target_pce").(string)
	} else if viper.Get("default_pce_name") != nil && viper.Get("default_pce_name").(string) != "" {
		name = viper.Get("default_pce_name").(string)
	} else {
		LogError("there is no pce set using the --pce flag and there is no default pce. either run workloader pce-add to add your first pce or workloader set-default to set an existing PCE as default.")
	}

	// Get the PCE
	pce, err := GetPCEbyName(name, GetLabelMaps)
	if err != nil {
		return illumioapi.PCE{}, err
	}
	return pce, nil
}


// GetPCEbyName gets a PCE by it's provided name
func GetPCEbyName(name string, GetLabelMaps bool) (illumioapi.PCE, error) {
	var pce illumioapi.PCE
	if viper.IsSet(name + ".fqdn") {
		pce = illumioapi.PCE{FriendlyName: name, FQDN: viper.Get(name + ".fqdn").(string), Port: viper.Get(name + ".port").(int), Org: viper.Get(name + ".org").(int), User: viper.Get(name + ".user").(string), Key: viper.Get(name + ".key").(string), DisableTLSChecking: viper.Get(name + ".disableTLSChecking").(bool)}
		if GetLabelMaps {
			if err := pce.Load(illumioapi.LoadInput{Labels: true}); err != nil {
				LogError(err.Error())
			}
		}
		return pce, nil
	}

	return illumioapi.PCE{}, fmt.Errorf("could not retrieve %s PCE information", name)
}
