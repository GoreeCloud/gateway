// Command gateway is the GoreeCloud Gateway distribution of the Caddy engine.
package main

import (
	"github.com/caddyserver/caddy/v2/cmd"
	_ "github.com/caddyserver/caddy/v2/modules/standard"
	_ "github.com/GoreeCloud/gateway/modules/gateway"
)

func main() {
	cmd.Main()
}
