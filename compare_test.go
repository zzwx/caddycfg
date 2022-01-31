package caddycfg

import "testing"

func TestRouteConfigsEqual(t *testing.T) {
	cfg0 := `
{
	"@id": "example.com",
	"handle": [
		{
			"transport": {
				"protocol": "http"
			},
			"handler": "reverse_proxy",
			"upstreams": [
				{
					"dial": "localhost:8080"
				}
			]
		}
	],
	"match": [
		{
			"host": [
				"example.com"
			],
			"path": [
				"/*"
			]
		}
	]
}`
	cfg1 := `
{
	"@id": "example.com",
	"handle": [
		{
			"transport": {
				"protocol": "http"
			},
			"handler": "reverse_proxy",
			"upstreams": [
				{
					"dial": "localhost:8080"
				}
			]
		}
	],
	"match": [
		{
			"host": [
				"example.com"
			],
			"path": [
				"/*"
			]
		}
	]
}`
	if !RouteConfigsEqual(cfg0, cfg1) {
		t.Errorf("Expected equal, found different:\n%v\n%v", cfg0, cfg1)
	}
	// spacing changes
	cfg1 = `{
	"@id": "example.com",

	"handle": [
		{
			"transport": {
				"protocol": "http"
			},
			"handler": "reverse_proxy",
			"upstreams": [
				{
					"dial": "localhost:8080"
				}
			]
		}
	],
	"match": [
		{
			"host": [
				"example.com"
			],
			"path": [
				"/*"
			]
		}
	]
}`
	if !RouteConfigsEqual(cfg0, cfg1) {
		t.Errorf("Expected equal, found different:\n%v\n%v", cfg0, cfg1)
	}
	// reshuffling of the fields
	cfg1 = `{
	"@id": "example.com",
	"match": [
		{
			"path": [
				"/*"
			],
			"host": [
				"example.com"
			]
		}
	],
	"handle": [

		{
			"upstreams": [
				{
					"dial": "localhost:8080"
				}
			],
			"handler": "reverse_proxy",
			"transport": {
				"protocol": "http"
			}
		}
	]
}`
	if !RouteConfigsEqual(cfg0, cfg1) {
		t.Errorf("Expected equal, found different:\n%v\n%v", cfg0, cfg1)
	}
}
