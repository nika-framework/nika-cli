// Command {{.AppName}} is a Nika microservice served over raw TCP, with no
// broker in the middle.
//
// There is no persistence, no fan-out and no queue to absorb a restart: if this
// process is down when a peer publishes, the message is gone. That is the right
// trade for a sidecar, a tightly coupled pair of services, and tests.
package main

import (
	"github.com/nika-framework/nika"
	"github.com/nika-framework/nika/common/config"
	"github.com/nika-framework/nika/common/microservice"
	"github.com/nika-framework/nika/common/microservice/transport/tcpmq"
	"github.com/nika-framework/nika/common/validator"

	"{{.ModulePath}}/{{.SrcImport}}"
)

func main() {
	app := nika.NewApp()

	validator.Setup(app)
	cfg := config.Setup(app, "{{.EnvPath}}")

	transport := tcpmq.MustNew(tcpmq.Options{
		// Addr is the bind address for this server and the default dial target
		// for its own client half; DialAddr overrides the latter.
		Addr:     cfg.GetString("TCP_ADDR", ":4000"),
		DialAddr: cfg.GetString("TCP_DIAL_ADDR", ""),
	})

	if _, err := microservice.Setup(app, microservice.Config{Transport: transport}); err != nil {
		panic(err)
	}

	app.LoadModule(src.NewAppModule())

	// RunWorker starts the listener, blocks until SIGINT/SIGTERM, then drains
	// in-flight handlers and closes the transport.
	if err := app.RunWorker(); err != nil {
		panic(err)
	}
}
