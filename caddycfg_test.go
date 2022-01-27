package caddycfg

import (
	"encoding/json"
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
	runWithinManagedDefaultCaddy(
		"http://localhost:20259",
		func(caddyInstance *CaddyHttpServerInstance) {
			serverKey := "myserver"
			err := caddyInstance.ReplaceRouteConfig(
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
			err = caddyInstance.ReplaceRouteConfig(
				serverKey,
				"some.example.com",
				ReverseProxyCaddyRouteConf(8081, []string{
					"some.example.com",
				}, "/*"),
			)
			if err != nil {
				t.Errorf("%v", err)
			}
			err = caddyInstance.ReplaceRouteConfig(
				serverKey,
				"game.example.org",
				ReverseProxyCaddyRouteConf(8082, []string{
					"game.example.org",
				}, "/*"),
			)
			if err != nil {
				t.Errorf("%v", err)
			}
			err = caddyInstance.ReplaceRouteConfig(
				serverKey,
				"go.example.com",
				ReverseProxyCaddyRouteConf(8083, []string{
					"go.example.com",
				}, "/*"),
			)
			if err != nil {
				t.Errorf("%v", err)
			}
			c, err := caddyInstance.Config()
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

func runWithinManagedDefaultCaddy(configURL string, f func(caddyInstance *CaddyHttpServerInstance)) {
	runWithinManagedCaddy("", configURL, f)
}

// runWithinManagedCaddy runs a caddy server instance ("caddy run") with optional "--config" file,
// then creates NewCaddyHttpServerInstance with the passed configURL, loads it with CaddyHttpServerInstance.EmptyConfig if no "--config" file is provided,
// and executes f with created instance.
//
// After finishing execution of f, it stops caddy server instance.
//
// With the sync.Mutex it makes sure to run just one caddy at a time per process.
func runWithinManagedCaddy(configFile string, configURL string, f func(caddyInstance *CaddyHttpServerInstance)) {
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
	var caddyInstance = NewCaddyHttpServerInstance(configURL)
	if configFile == "" {
		err := caddyInstance.UploadConfigToDefault(caddyInstance.EmptyConfig("myserver"))
		if err != nil {
			panic(err)
		}
	}

	f(caddyInstance)
	if err := cmd.Process.Kill(); err == nil {
		err := cmd.Wait()
		if err != nil {
			// fmt.Printf("non-critical error of process wait: %v\n", err)
		}
	} else {
		fmt.Printf("non-critical error of process kill: %v\n", err)
	}
}

func TestCaddyHttpServerInstance_ReplaceRouteConfig(t *testing.T) {
	runWithinManagedCaddy(
		"./test/caddy_config_base_with_endpoint.json",
		"http://localhost:20247",
		func(caddyInstance *CaddyHttpServerInstance) {
			err := caddyInstance.DeleteConfigById("test")
			if err == nil {
				t.Errorf("Expected not found error, got %v", nil)
			}
			for i := 0; i < 5; i++ {
				id := "example.com"
				m := ReverseProxyCaddyRouteConf(
					8080,
					[]string{
						fmt.Sprintf("%d.example.com", i),
					}, "/*",
				)
				err := caddyInstance.ReplaceRouteConfig("myserver", id, m)
				if err != nil {
					fmt.Printf("error: %v", err)
				}
			}
			c, err := caddyInstance.Config()
			if err != nil {
				t.Errorf("%v", err)
			}
			want := `{"admin":{"listen":"localhost:20247"},"apps":{"http":{"servers":{"myserver":{"automatic_https":{"skip":[]},"listen":[":443"],"routes":[{"@id":"example.com","handle":[{"handler":"reverse_proxy","transport":{"protocol":"http"},"upstreams":[{"dial":"localhost:8080"}]}],"match":[{"host":["4.example.com"],"path":["/*"]}]}]}}}}}`
			if c != want {
				t.Errorf("Config error, want:\n%v\ngot:\n%v", want, c)
			}
		})
}

func printConfig(caddyInstance *CaddyHttpServerInstance) {
	fmt.Printf("-----config-----\n")
	c, err := caddyInstance.Config()
	if err != nil {
		panic(err)
	}
	fmt.Println(c)
}

func printConfigById(caddyInstance *CaddyHttpServerInstance, id string) {
	fmt.Printf("-----config %v-----\n", id)
	c, err := caddyInstance.ConfigById(id)
	if err != nil {
		fmt.Printf("error: %v\n", err)
	}
	fmt.Println(c)
}
