package caddycfg

import (
	"encoding/json"
	"reflect"
	"strings"

	"github.com/antlr/antlr4/runtime/Go/antlr"
)

func _unsuedFunction() {
	var _ antlr.ATNConfig
}

// RouteConfigsEqual compares two configurations that can
// be decoded into RouteConfigType, allowing for shuffled
// named parameters.
func RouteConfigsEqual(cfg0, cfg1 string) bool {
	if cfg0 == cfg1 {
		// This is rare that they both simply be equal like this, as they seem to be marshalled differently by Caddy
		// when configuration is requested vs when it's pushed.
		return true
	}
	// We have to try to compare them structurally.
	decCfg := json.NewDecoder(strings.NewReader(cfg0))
	var cfgJson RouteConfigType
	err := decCfg.Decode(&cfgJson)
	if err == nil {
		decCurrent := json.NewDecoder(strings.NewReader(cfg1))
		var currentJson RouteConfigType
		err := decCurrent.Decode(&currentJson)
		if err == nil {
			// Now we can compare two objects
			if reflect.DeepEqual(cfgJson, currentJson) {
				return true
			}
		}
	}
	return false
}

// RouteConfigType is used to compare route configurations.
type RouteConfigType struct {
	// TODO: It must eventually grow to fill the gaps
	Id    string `json:"@id"`
	Match []struct {
		Host []string `json:"host"`
		Path []string `json:"path"`
	} `json:"match"`
	Handle []struct {
		Handler   string `json:"handler"`
		Transport struct {
			Protocol string `json:"protocol"`
		} `json:"transport"`
		Upstreams []struct {
			Dial string `json:"dial"`
		} `json:"upstreams"`
	} `json:"handle"`
}
