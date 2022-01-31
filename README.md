# caddycfg

Tiny library to modify `caddy` server's configuration from the web sites themselves.

## Installation

`go get github.com/zzwx/caddycfg@master`

## Motivation

Playing with `caddy` global configuration made me feel that it would be cool for them to be modular. There might be better solutions for this workflow, but this is what works for my small projects.

It's a work-in-progress project. Feel free to comment or enhance with pull requests.

## Tests Notes

The tests assume that the `caddy` server is available.

## Usage Example

```go
func runCaddyConfRefresher() {
	if !cfg.CaddyCfg.Enabled {
		fmt.Printf("Skipping [caddycfg] injection due to [caddycfg].enabled = false")
		return
	}
	fmt.Printf("[caddycfg] injection enabled!")
	modification := func() {
		instance := caddycfg.NewCaddyCfg(caddycfg.DefaultConfigURL)
		err := instance.AddRoute(
			cfg.CaddyCfg.ServerKey,
			cfg.CaddyCfg.RouteId,
			caddycfg.ReverseProxyCaddyRouteConf(
				cfg.Server.Port,
				cfg.CaddyCfg.MatchHosts,
				cfg.CaddyCfg.PathMatch,
			))
		if err != nil {
			fmt.Printf("error changing Caddy configuration: %v\n", err)
		}
	}
	// This ensures that if Caddy is restarted or initiated later than the app,
	// configuration will still reach it within this max interval.
	go caddycfg.Refresher(time.Second*4, modification)
}
```

