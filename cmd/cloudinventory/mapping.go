package cloudinventory

import (
	"encoding/json"
	"log"
	"net/http"
	"time"
)

type Mappings struct {
	UpdatedDate time.Time `json:"updatedDate"`
	Version     int       `json:"version"`
	Clouds      Clouds    `json:"clouds"`
}
type Aws struct {
	ResourceName           string  `json:"resource_name"`
	ScriptResourceName     string  `json:"script_resource_name"`
	ResourceType           string  `json:"resource_type"`
	ResourceTypeHyphenated string  `json:"resource_type_hyphenated"`
	RatioIwl               float64 `json:"ratio_iwl"`
	RatioSwl               float64 `json:"ratio_swl"`
	EnabledInsights        bool    `json:"enabled_insights"`
	EnabledSegmentation    bool    `json:"enabled_segmentation"`
	APICommand             string  `json:"api_command"`
	CountQuery             string  `json:"count_query"`
	IsGlobal               bool    `json:"is_global,omitempty"`
	ContainerHostTag       string  `json:"container_host_tag,omitempty"`
}
type Gcp struct {
	ResourceName           string  `json:"resource_name"`
	ScriptResourceName     string  `json:"script_resource_name"`
	ResourceType           string  `json:"resource_type"`
	ResourceTypeHyphenated string  `json:"resource_type_hyphenated"`
	RatioIwl               float64 `json:"ratio_iwl"`
	RatioSwl               float64 `json:"ratio_swl"`
	EnabledInsights        bool    `json:"enabled_insights"`
	EnabledSegmentation    bool    `json:"enabled_segmentation"`
}
type Azure struct {
	ResourceName           string  `json:"resource_name"`
	ScriptResourceName     string  `json:"script_resource_name"`
	ResourceType           string  `json:"resource_type"`
	ResourceTypeHyphenated string  `json:"resource_type_hyphenated"`
	RatioIwl               float64 `json:"ratio_iwl"`
	RatioSwl               float64 `json:"ratio_swl"`
	EnabledInsights        bool    `json:"enabled_insights"`
	EnabledSegmentation    bool    `json:"enabled_segmentation"`
	IsChild                bool    `json:"is_child"`
	ParentType             string  `json:"parent_type,omitempty"`
	ContainerHostTag       string  `json:"container_host_tag,omitempty"`
}
type Clouds struct {
	Aws   []Aws   `json:"aws"`
	Gcp   []Gcp   `json:"gcp"`
	Azure []Azure `json:"azure"`
}

func getMappings() (segmenationMaps, insightsMaps map[string]float64, resourceToCloudMaps map[string]string) {
	mappingUrl := "https://cloudsecure-onboarding-templates.s3.us-west-2.amazonaws.com/cloudsecure/resources.json"

	// Get mappings from URL
	resp, err := http.Get(mappingUrl)
	if err != nil {
		log.Fatalf("failed to get mappings: %s", err)
	}

	// Marshall response
	defer resp.Body.Close()
	var mappings Mappings
	if err := json.NewDecoder(resp.Body).Decode(&mappings); err != nil {
		log.Fatalf("failed to decode mappings: %s", err)
	}

	// create maps
	segmenationMaps = make(map[string]float64)
	insightsMaps = make(map[string]float64)
	resourceToCloudMaps = make(map[string]string)

	for _, resource := range mappings.Clouds.Aws {
		resourceToCloudMaps[resource.ResourceName] = "aws"
		if resource.EnabledSegmentation {
			segmenationMaps[resource.ResourceName] = resource.RatioSwl
		}
		if resource.EnabledInsights {
			insightsMaps[resource.ResourceName] = resource.RatioIwl
		}
	}
	for _, resource := range mappings.Clouds.Azure {
		resourceToCloudMaps[resource.ResourceName] = "azure"
		if resource.EnabledSegmentation {
			segmenationMaps[resource.ResourceName] = resource.RatioSwl
		}
		if resource.EnabledInsights {
			insightsMaps[resource.ResourceName] = resource.RatioIwl
		}
	}
	for _, resource := range mappings.Clouds.Gcp {
		resourceToCloudMaps[resource.ResourceName] = "gcp"
		if resource.EnabledSegmentation {
			segmenationMaps[resource.ResourceName] = resource.RatioSwl
		}
		if resource.EnabledInsights {
			insightsMaps[resource.ResourceName] = resource.RatioIwl
		}
	}

	return segmenationMaps, insightsMaps, resourceToCloudMaps
}
