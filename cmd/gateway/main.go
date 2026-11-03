// Command gateway is the GoreeCloud Gateway distribution of the Caddy engine.
package main

import (
	_ "github.com/GoreeCloud/gateway/modules/gateway"
	"github.com/caddyserver/caddy/v2/cmd"
	_ "github.com/caddyserver/caddy/v2/modules/standard"
)

func main() {
	caddycmd.Main()
}
