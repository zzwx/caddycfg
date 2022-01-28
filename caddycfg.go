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
	"path"
	"reflect"
	"strconv"
	"strings"
)

const DefaultConfigURL = "http://localhost:2019"

type CaddyCfg struct {
	configURL httpcaddyfile.Address
}

// NewCaddyCfg accepts caddy's configuration url as string.
//
// For default "http://localhost:2019" configuration use NewCaddyCfg(DefaultConfigURL).
func NewCaddyCfg(configURL string) *CaddyCfg {
	addrr, err := httpcaddyfile.ParseAddress(configURL)
	if err != nil {
		panic(err) // Well, I justfiy panicing here! Easy to catch and fix.
	}
	addr := addrr.String() // this adds "http"
	reparsed, _ := httpcaddyfile.ParseAddress(addr)
	return &CaddyCfg{
		configURL: reparsed,
	}
}

var (
	// ErrNotFoundID is a base error to what errNotFoundID leads to when unwrapped,
	// in order to check with errors.Is(err, ErrNotFoundID)
	ErrNotFoundID = errors.New("unknown object ID")
)

// errNotFoundID stands for errors of not found object id,
// it contains id when printed with Error, and unwraps to ErrNotFoundID to be checked using
// errors.Is(err, ErrNotFoundID)
type errNotFoundID struct {
	id string
}

func newNotFoundID(id string) *errNotFoundID {
	return &errNotFoundID{
		id: id,
	}
}

func (n errNotFoundID) Error() string {
	return fmt.Sprintf("not found ID '%v'", n.id)
}

func (n errNotFoundID) Unwrap() error {
	return ErrNotFoundID
}

const messageErrorUnknownObjectIDPrefix = "{\"error\":\"unknown object ID"

// UploadTo does the same as Upload, only to a custom configURL, which is usually DefaultConfigURL.
// This is to allow to upload a new configuration on top of an empty `caddy run` that started with a 'null' configuration.
//
// If configJSON contains a new "admin:listen" section, it seems to retarget caddy's configURL to it for any next configuration manipulations.
func (caddyCfg *CaddyCfg) UploadTo(configURL string, configJSON string) error {
	r, err := http.Post(JoinURLPath(configURL, "load"), "application/json", strings.NewReader(configJSON))
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

// Upload (in Caddy terms "load") is sending full configuration that will replace
// the existing one completely. It might be good for a base configuration.
func (caddyCfg *CaddyCfg) Upload(configJSON string) error {
	return caddyCfg.UploadTo(caddyCfg.configURL.String(), configJSON)
}

// Config returns full configuration of CaddyCfg, including
// root node. Trailing "\n" will be removed.
func (caddyCfg *CaddyCfg) Config() (string, error) {
	loadConfig, err := http.Get(JoinURLPath(caddyCfg.configURL.String(), "config"))
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
// If not finding the object by id error occurs, it will be converted into a errNotFoundID.
func (caddyCfg *CaddyCfg) ConfigById(id string) (string, error) {
	loadConfig, err := http.Get(JoinURLPath(caddyCfg.configURL.String(), "id", url.PathEscape(id)))
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
		return "", newNotFoundID(id)
	}
	return strings.TrimSuffix(s, "\n"), nil
}

// DeleteById attempts to delete a config by specified id. In theory this should work
// for any section of configuration, but here it's only used to remove routes.
//
// If not finding the object by id error occurs, it will be converted into a errNotFoundID.
func (caddyCfg *CaddyCfg) DeleteById(id string) error {
	client := http.DefaultClient
	configUrl := JoinURLPath(caddyCfg.configURL.String(), "id", url.PathEscape(id))

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
		return newNotFoundID(id)
	}
	return nil
}

type IDField struct {
	Id string `json:"@id"`
}

type RoutConfigType struct {
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

// AddRoute ensures that configuration marked by unique route config "@id" field specified by routeId enters caddy's configuration.
// A good candidate for routeId is a domain name.
//
// This function first pokes caddy for current configuration on "@id" to see if it matches passed routeConfig byte-to-byte.
// If it does, it simply skips the change, otherwise deletes configuration (ignoring errors) and then adds it again, to keep only
// one configuration for this route "@id".
//
// serverKey is arbitrary name in the base configuration for the http/servers enty. By default it's usually "myserver". Lookup base
// configuration for the right key.
//
//	{
//		"apps": {
//			"http": {
//				"servers": {
//					"<serverKey>":
func (caddyCfg *CaddyCfg) AddRoute(serverKey string, routeId string, routeConfig *caddyhttp.Route) error {
	config, err := json.Marshal(routeConfig)
	if err != nil {
		return err
	}

	cfg := string(config)

	// Prepend config with "@id" by brutally forcing it into JSON, as caddyhttp.Route has no
	// field for it.
	cfg = strings.Replace(cfg, "{", fmt.Sprintf("{%v,", EncodeAtId(routeId)), 1)

	current, err := caddyCfg.ConfigById(routeId)
	if err == nil { // including errNotFoundID
		if cfg == current {
			// This is rare that they both simply be equal, as they seem to be marshalled differently by Caddy
			// when requested.
			return nil
		} else {
			// We have to try to compare them structurally.
			decCfg := json.NewDecoder(strings.NewReader(cfg))
			var cfgJson RoutConfigType
			err := decCfg.Decode(&cfgJson)
			if err == nil {
				decCurrent := json.NewDecoder(strings.NewReader(current))
				var currentJson RoutConfigType
				err := decCurrent.Decode(&currentJson)
				if err == nil {
					// Now we can compare two objects
					if reflect.DeepEqual(cfgJson, currentJson) {
						return nil
					}
				}
			}
		}
	}

	if current != "" {
		_ = caddyCfg.DeleteById(routeId)
	}

	requestPatch, err := http.Post(
		JoinURLPath(
			caddyCfg.configURL.String(),
			"config", "apps", "http", "servers", url.PathEscape(serverKey), "/routes/",
		),
		"application/json",
		strings.NewReader(cfg),
	)
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

// EncodeJSONString returns "value" properly escaped for JSON and surrounded with quotes.
func EncodeJSONString(value string) string {
	return strings.TrimPrefix(EncodeAtId(value), `"@id":`)
}

// BaseConfig returns a JSON string of a base configuration with :443 listen port and empty routes array:
//
//	"admin": { "listen": <caddyCfg.configURL> { ...
//	"apps"."http"."servers" { "<serverKey>": ...
//	                          "listen": [":443"]
//                            "routes": []
//
// This can be passed to CaddyCfg.Upload as initial empty configuration
// that might be later enhanced with routes.
func BaseConfig(configURL string, serverKey string) string {
	c := NewCaddyCfg(configURL)
	// "listen" doesn't like http:// or https://
	address := c.configURL
	address.Scheme = ""
	str := address.String() // still adds http:// or https://
	str = strings.TrimPrefix(str, "http://")
	str = strings.TrimPrefix(str, "https://")
	return `{
	"admin": {
		"listen": ` + EncodeJSONString(str) + `
	},
	"apps": {
		"http": {
			"servers": {
				` + EncodeJSONString(serverKey) + `: {
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
}

// JoinURLPath ignores any url_ parsing errors
func JoinURLPath(url_ string, paths ...string) string {
	u, err := url.Parse(url_)
	if err != nil {
		return strings.TrimSuffix(url_, "/") + "/" + path.Join(paths...)
	}
	u.Path = path.Join(append([]string{u.Path}, paths...)...)
	return u.String()
}
