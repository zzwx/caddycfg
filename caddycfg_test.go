package caddycfg

import (
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"sync"
	"testing"
)

func ExampleEncodeAtId() {
	stripped := EncodeAtId("test")
	fmt.Println(stripped)
	// Output:
	// "@id":"test"
}

func TestReverseProxyCaddyRouteConf(t *testing.T) {
	runWithinManagedEmptyCaddy(
		"http://localhost:20259", "myserver",
		func(caddyCfg *CaddyCfg) {
			serverKey := "myserver"
			err := caddyCfg.AddRoute(
				serverKey,
				"example.net",
				ReverseProxyCaddyRouteConf(8080, []string{
					"example.net",
					"www.example.net",
				}, "/*"),
			)
			if err != nil {
				t.Errorf("%v", err)
			}
			err = caddyCfg.AddRoute(
				serverKey,
				"some.example.com",
				ReverseProxyCaddyRouteConf(8081, []string{
					"some.example.com",
				}, "/*"),
			)
			if err != nil {
				t.Errorf("%v", err)
			}
			err = caddyCfg.AddRoute(
				serverKey,
				"game.example.org",
				ReverseProxyCaddyRouteConf(8082, []string{
					"game.example.org",
				}, "/*"),
			)
			if err != nil {
				t.Errorf("%v", err)
			}
			err = caddyCfg.AddRoute(
				serverKey,
				"go.example.com",
				ReverseProxyCaddyRouteConf(8083, []string{
					"go.example.com",
				}, "/*"),
			)
			if err != nil {
				t.Errorf("%v", err)
			}
			c, err := caddyCfg.Config()
			if err != nil {
				t.Errorf("%v", err)
			}
			want := `{"admin":{"listen":"localhost:20259"},"apps":{"http":{"servers":{"myserver":{"automatic_https":{"skip":[]},"listen":[":443"],"routes":[{"@id":"example.net","handle":[{"handler":"reverse_proxy","transport":{"protocol":"http"},"upstreams":[{"dial":"localhost:8080"}]}],"match":[{"host":["example.net","www.example.net"],"path":["/*"]}]},{"@id":"some.example.com","handle":[{"handler":"reverse_proxy","transport":{"protocol":"http"},"upstreams":[{"dial":"localhost:8081"}]}],"match":[{"host":["some.example.com"],"path":["/*"]}]},{"@id":"game.example.org","handle":[{"handler":"reverse_proxy","transport":{"protocol":"http"},"upstreams":[{"dial":"localhost:8082"}]}],"match":[{"host":["game.example.org"],"path":["/*"]}]},{"@id":"go.example.com","handle":[{"handler":"reverse_proxy","transport":{"protocol":"http"},"upstreams":[{"dial":"localhost:8083"}]}],"match":[{"host":["go.example.com"],"path":["/*"]}]}]}}}}}`
			if c != want {
				t.Errorf("Config error, want:\n%v\ngot:\n%v", want, c)
			}
		})
}

func ExampleReverseProxyCaddyRouteConf() {
	r := ReverseProxyCaddyRouteConf(
		8080,
		[]string{
			"example.com",
			"www.example.com",
		}, "/*",
	)
	s, err := json.MarshalIndent(r, "", "\t")
	if err != nil {
		panic(err)
	}
	fmt.Println(string(s))

	//Output:
	//{
	//	"match": [
	//		{
	//			"host": [
	//				"example.com",
	//				"www.example.com"
	//			],
	//			"path": [
	//				"/*"
	//			]
	//		}
	//	],
	//	"handle": [
	//		{
	//			"handler": "reverse_proxy",
	//			"transport": {
	//				"protocol": "http"
	//			},
	//			"upstreams": [
	//				{
	//					"dial": "localhost:8080"
	//				}
	//			]
	//		}
	//	]
	//}
}

var m sync.Mutex

// runWithinManagedEmptyCaddy calls runWithinManagedCaddy with empty configFile,
// to run empty caddy, that will be enhanced with CaddyCfg.EmptyConfig(serverKey).
//
// configURL will become new configuration URL once caddy gets CaddyCfg.EmptyConfig configuration, to call f()
// with the caddyCfg.
func runWithinManagedEmptyCaddy(configURL string, serverKey string, f func(caddyCfg *CaddyCfg)) {
	runWithinManagedCaddy("", configURL, serverKey, f)
}

// runWithinManagedCaddy runs a caddy server instance ("caddy run --config configFile"),
// then creates NewCaddyCfg with the passed configURL and executes f with created *CaddyCfg.
//
// After finishing execution of f, it stops caddy server.
//
// A special case with configFile == "" is to loads empty caddy ("caddy run") with CaddyCfg.EmptyConfig(serverKey) if no "--config" file is provided,
//
//
//
// With the sync.Mutex it makes sure to run just one caddy server at a time per process.
func runWithinManagedCaddy(configFile string, configURL string, serverKey string, f func(caddyCfg *CaddyCfg)) {
	m.Lock()
	defer m.Unlock()
	config := []string{"run"}
	if configFile != "" {
		config = append(config, "--config", configFile)
	}
	cmd := exec.Command("caddy", config...)
	err := cmd.Start()
	if err != nil {
		panic(err)
	}
	var caddyCfg = NewCaddyCfg(configURL)
	if configFile == "" {
		base := BaseConfig(configURL, serverKey)
		err := caddyCfg.UploadTo(DefaultConfigURL, base)
		if err != nil {
			panic(err)
		}
	}

	f(caddyCfg)
	if err := cmd.Process.Kill(); err == nil {
		err := cmd.Wait()
		if err != nil {
			// fmt.Printf("non-critical error of process wait: %v\n", err)
		}
	} else {
		fmt.Printf("non-critical error of process kill: %v\n", err)
	}
}

func TestCaddyCfg_AddRouteConfig_Overwrite(t *testing.T) {
	runWithinManagedEmptyCaddy("localhost:2019", "myserver", func(caddyCfg *CaddyCfg) {

	})
}

func TestCaddyCfg_AddRouteConfig(t *testing.T) {
	runWithinManagedCaddy(
		"./test/caddy_config_base_with_endpoint.json",
		"http://localhost:20247",
		"myserver",
		func(caddyCfg *CaddyCfg) {
			err := caddyCfg.DeleteById("test")
			if err == nil {
				t.Errorf("Expected not found error, got %v", nil)
			} else if !errors.Is(err, ErrNotFoundID) {
				t.Errorf("Expected not found error, got %v", err)
			}
			for i := 0; i < 5; i++ {
				id := "example.com"
				m := ReverseProxyCaddyRouteConf(
					8080,
					[]string{
						fmt.Sprintf("%d.example.com", i),
					}, "/*",
				)
				err := caddyCfg.AddRoute("myserver", id, m)
				if err != nil {
					fmt.Printf("error: %v", err)
				}
			}
			c, err := caddyCfg.Config()
			if err != nil {
				t.Errorf("%v", err)
			}
			want := `{"admin":{"listen":"localhost:20247"},"apps":{"http":{"servers":{"myserver":{"automatic_https":{"skip":[]},"listen":[":443"],"routes":[{"@id":"example.com","handle":[{"handler":"reverse_proxy","transport":{"protocol":"http"},"upstreams":[{"dial":"localhost:8080"}]}],"match":[{"host":["4.example.com"],"path":["/*"]}]}]}}}}}`
			if c != want {
				t.Errorf("Config error, want:\n%v\ngot:\n%v", want, c)
			}
		})
}

func printConfig(caddyCfg *CaddyCfg) {
	fmt.Printf("-----config-----\n")
	c, err := caddyCfg.Config()
	if err != nil {
		panic(err)
	}
	fmt.Println(c)
}

func printConfigById(caddyCfg *CaddyCfg, id string) {
	fmt.Printf("-----config %v-----\n", id)
	c, err := caddyCfg.ConfigById(id)
	if err != nil {
		fmt.Printf("error: %v\n", err)
	}
	fmt.Println(c)
}

func ExampleJoinURLPath() {
	fmt.Println(JoinURLPath("http://localhost:2019", "test"))
	fmt.Println(JoinURLPath("http://localhost:2019/", "test"))
	fmt.Println(JoinURLPath("http://localhost:2019/in", "test", "where", "to", "go"))
	fmt.Println(JoinURLPath("", "test"))
	// Output:
	// http://localhost:2019/test
	// http://localhost:2019/test
	// http://localhost:2019/in/test/where/to/go
	// test
}
