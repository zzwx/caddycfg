package caddycfg

import "testing"

func TestRouteConfigsEqual(t *testing.T) {
	cfg0 := `{"admin":{"listen":"localhost:20247"},
"apps":{"http":{"servers":{"myserver":{"automatic_https":{"skip":[]},"listen":[":443"],"routes":[{"@id":"example.com","handle":[{"handler":"reverse_proxy","transport":{"protocol":"http"},"upstreams":[{"dial":"localhost:8080"}]}],"match":[{"host":["4.example.com"],"path":["/*"]}]}]}}}}}`
	cfg1 := `{"admin":{"listen":"localhost:20247"},

"apps":{"http":{"servers":{"myserver":{"automatic_https":{"skip":[]},"listen":[":443"],"routes":[{"@id":"example.com","handle":[{"handler":"reverse_proxy","transport":{"protocol":"http"},"upstreams":[{"dial":"localhost:8080"}]}],"match":[{"host":["4.example.com"],"path":["/*"]}]}]}}}}}`
	if !RouteConfigsEqual(cfg0, cfg1) {
		t.Errorf("Expected equal, found not:\n%v\n%v", cfg0, cfg1)
	}
	cfg1 = `{"admin":{"listen":"localhost:20247"},

"apps":
	{"http":{"servers":{"myserver":
		{"automatic_https":{"skip":[]},
		"listen":[":443"],
		"routes":[
			{"@id":"example.com",
			 "handle":[{"handler":"reverse_proxy","transport":{"protocol":"http"},
       "upstreams":[{"dial":"localhost:8080"}]}],"match":[{"host":["4.example.com"],"path":["/*"]}
        ]}]}}
}
}}`
	if !RouteConfigsEqual(cfg0, cfg1) {
		t.Errorf("Expected equal, found not:\n%v\n%v", cfg0, cfg1)
	}
	cfg1 = `{"admin":{"listen":"localhost:20247"}, "apps":
	{"http":{"servers":{"myserver":
		{"automatic_https":{"skip":[]},
		"listen":[":443"],
		"routes":[
			{"@id":"example.com",
			 "handle":[{"transport":{"protocol":"http"},"handler":"reverse_proxy",
       "upstreams":[{"dial":"localhost:8080"}]}],"match":[{"host":["4.example.com"],"path":["/*"]}
        ]}]}}
}
}}`
	if !RouteConfigsEqual(cfg0, cfg1) {
		t.Errorf("Expected equal, found not:\n%v\n%v", cfg0, cfg1)
	}

}
