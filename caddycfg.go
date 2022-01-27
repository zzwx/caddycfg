package caddycfg

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig"
	"github.com/caddyserver/caddy/v2/caddyconfig/httpcaddyfile"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp/reverseproxy"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

type CaddyHttpServerInstance struct {
	configURL string // We trim trailing "/" for consistency
}

// NewCaddyHttpServerInstance accepts caddy's configuration url,
// by default "http://localhost:2019".
func NewCaddyHttpServerInstance(configURL string) *CaddyHttpServerInstance {
	configURL = strings.TrimSuffix(configURL, "/")
	return &CaddyHttpServerInstance{
		configURL: configURL,
	}
}

type NotFoundIDError struct {
	id string
}

func NewNotFoundError(id string) *NotFoundIDError {
	return &NotFoundIDError{
		id: id,
	}
}

func (n NotFoundIDError) Error() string {
	return fmt.Sprintf("not found ID '%v'", n.id)
}

const messageErrorUnknownObjectIDPrefix = "{\"error\":\"unknown object ID"

// EmptyConfig returns a JSON string of a base configuration with:
//
//	admin: { "listen": <caddyInstance.configURL> { ...
//
// and
//
//	apps.http.servers { "<serverKey>":
//
// This can be passed to CaddyHttpServerInstance.UploadConfig as initial empty configuration
// that might be later enhanced with routes.
func (caddyInstance *CaddyHttpServerInstance) EmptyConfig(serverKey string) string {
	// listen doesn't like http:// or https://
	address, err := httpcaddyfile.ParseAddress(caddyInstance.configURL)
	if err != nil {
		return ""
	}

	var v = `
{
	"admin": {
		"listen": ` + EncodeJSONValue(address.Host+":"+address.Port) + `
	},
	"apps": {
		"http": {
			"servers": {
				` + EncodeJSONValue(serverKey) + `: {
					"automatic_https": {
						"skip": []
					},
					"listen": [
						":443"
					],
					"routes": []
				}
			}
		}
	}
}
`
	return v
}

func (caddyInstance *CaddyHttpServerInstance) uploadConfigTo(configURL string, configJSON string) error {
	r, err := http.Post(configURL+"/load", "application/json", strings.NewReader(configJSON))
	if err != nil {
		return err
	}
	defer r.Body.Close()
	b, err := io.ReadAll(r.Body)
	if err != nil {
		return err
	}
	v := string(b)
	if v != "" {
		return errors.New(v)
	}
	return nil
}

// UploadConfigToDefault does the same as UploadConfig but to a "http://localhost:2019" instead of configured in
// caddyInstance. This is to allow to upload on top of initial `caddy run` without --config file, that
// always uses port 2019.
func (caddyInstance *CaddyHttpServerInstance) UploadConfigToDefault(configJSON string) error {
	return caddyInstance.uploadConfigTo("http://localhost:2019", configJSON)
}

// UploadConfig (in Caddy terms "load") is sending full configuration that will replace
// the existing one completely. It might be good for a begin configuration.
func (caddyInstance *CaddyHttpServerInstance) UploadConfig(configJSON string) error {
	return caddyInstance.uploadConfigTo(caddyInstance.configURL, configJSON)
}

// Config returns full configuration of CaddyHttpServerInstance, including
// root node. Trailing "\n" will be removed.
func (caddyInstance *CaddyHttpServerInstance) Config() (string, error) {
	loadConfig, err := http.Get(caddyInstance.configURL + "/config/")
	if err != nil {
		return "", err
	}
	defer loadConfig.Body.Close()
	b, err := io.ReadAll(loadConfig.Body)
	if err != nil {
		return "", err
	}
	return strings.TrimSuffix(string(b), "\n"), nil
}

// ConfigById returns configurtation section belonging to a marked by "@id" section in a JSON string format. Trailing "\n" will be removed.
//
// If not finding the object by id error occurs, it will be converted into a NotFoundIDError.
func (caddyInstance *CaddyHttpServerInstance) ConfigById(id string) (string, error) {
	loadConfig, err := http.Get(caddyInstance.configURL + "/id/" + url.PathEscape(id))
	if err != nil {
		return "", err
	}
	defer loadConfig.Body.Close()
	b, err := io.ReadAll(loadConfig.Body)
	if err != nil {
		return "", err
	}
	s := string(b)

	if strings.HasPrefix(s, messageErrorUnknownObjectIDPrefix) {
		return "", NewNotFoundError(id)
	}
	return strings.TrimSuffix(s, "\n"), nil
}

// DeleteConfigById attempts to delete config by specified id.
//
// If not finding the object by id error occurs, it will be converted into a NotFoundIDError.
func (caddyInstance *CaddyHttpServerInstance) DeleteConfigById(id string) error {
	client := http.DefaultClient
	configUrl := caddyInstance.configURL + "/id/" + url.PathEscape(id)

	req, err := http.NewRequest(http.MethodDelete, configUrl, bytes.NewBuffer(nil))
	//req.Header.Set("Content-Type", "application/json")
	//if err != nil {
	//	log.Fatal(err)
	//}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	s := string(b)
	if strings.HasPrefix(s, messageErrorUnknownObjectIDPrefix) {
		return NewNotFoundError(id)
	}
	return nil
}

type IDField struct {
	Id string `json:"@id"`
}

// ReplaceRouteConfig replaces configuration marked by unique route config "@id" field specified by routeId.
// A good candidate for routeId could be domain name.
//
// This function first deletes configuration (ignoring errors) and then adds it again, to keep only one configuration for this route.
//
// serverKey is arbitrary name in the base configuration for the http/servers enty. By default it's usually "myserver". Lookup base
// configuration for the right key.
//
//	{
//		"apps": {
//			"http": {
//				"servers": {
//					"<serverKey>":
func (caddyInstance *CaddyHttpServerInstance) ReplaceRouteConfig(serverKey string, routeId string, routeConfig *caddyhttp.Route) error {
	_ = caddyInstance.DeleteConfigById(routeId)
	config, err := json.Marshal(routeConfig)
	if err != nil {
		return err
	}

	cfg := string(config)

	// Prepend config with "@id" by brutally forcing it into JSON
	cfg = strings.Replace(cfg, "{", "{\n\t"+EncodeAtId(routeId)+",", 1)

	requestPatch, err := http.Post(caddyInstance.configURL+"/config/apps/http/servers/"+
		url.PathEscape(serverKey)+"/routes/", "application/json", strings.NewReader(cfg))
	if err != nil {
		return err
	}
	defer requestPatch.Body.Close()
	_, err = io.ReadAll(requestPatch.Body) // Returns nothing
	if err != nil {
		return err
	}
	return nil
}

// ReverseProxyCaddyRouteConf generates a "routes" (https://caddyserver.com/docs/json/apps/http/servers/routes/) element configuration structure.
// Returned route may be consumed as-is in the next steps or marshalled for Caddy using either json.Marshal or json.MarshalIndent:
//
//	m, err := json.MarshalIndent(route, "", "\t")
//
// pathMatch is usually "/*" for matching any paths.
func ReverseProxyCaddyRouteConf(backendPort int, matchHosts []string, pathMatch string) *caddyhttp.Route {
	toAddr, _ := httpcaddyfile.ParseAddress("localhost:" + strconv.Itoa(backendPort))
	ht := reverseproxy.HTTPTransport{}
	handler := reverseproxy.Handler{
		TransportRaw: caddyconfig.JSONModuleObject(ht, "protocol", "http", nil),
		Upstreams:    reverseproxy.UpstreamPool{{Dial: net.JoinHostPort(toAddr.Host, toAddr.Port)}},
	}
	route := caddyhttp.Route{
		HandlersRaw: []json.RawMessage{
			caddyconfig.JSONModuleObject(handler, "handler", "reverse_proxy", nil),
		},
	}
	route.MatcherSetsRaw = []caddy.ModuleMap{
		{
			"host": caddyconfig.JSON(caddyhttp.MatchHost(matchHosts), nil),
			"path": caddyconfig.JSON(caddyhttp.MatchPath{pathMatch}, nil),
		},
	}
	return &route
}

// EncodeAtId returns "@id":"<id>" encoded with
// proper character escaping for the id field.
func EncodeAtId(id string) string {
	var s strings.Builder
	e := json.NewEncoder(&s)
	err := e.Encode(IDField{Id: id})
	if err != nil {
		return ""
	}
	stripped := strings.TrimSuffix(s.String(), "}\n")
	stripped = strings.TrimPrefix(stripped, "{")
	return stripped
}

// EncodeJSONValue returns "value" properly escaped for JSON and surrounded with quotes.
func EncodeJSONValue(value string) string {
	return strings.TrimPrefix(EncodeAtId(value), `"@id":`)
}
