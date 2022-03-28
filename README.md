# caddycfg

A small library that helps modify [`Caddy`](https://caddyserver.com/) server's configuration from a web app.

## Installation

`go get -u github.com/zzwx/caddycfg`

## Motivation

Maintaining `Caddy` global configuration for several web apps may become cumbersome. Having each web app directly configure `Caddy` makes it possible to completely stop thinking about updating `Caddy` manually.

This is a work-in-progress project. Feel free to comment or enhance with pull requests.

## Tests Notes

The tests assume that a `Caddy` server is running on the testing site.

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

