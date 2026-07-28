// Command {{.AppName}} is a Nika microservice served over NATS.
//
// NATS_QUEUE_GROUP decides the most consequential behaviour here: set, replicas
// join a queue group and each message reaches exactly one of them (a
// load-balanced service); empty, every replica receives every message (a
// broadcast, so any non-idempotent handler runs once per replica).
package main

import (
	"github.com/nika-framework/nika"
	"github.com/nika-framework/nika/common/config"
	"github.com/nika-framework/nika/common/microservice"
	"github.com/nika-framework/nika/common/microservice/transport/natsmq"
	"github.com/nika-framework/nika/common/validator"

	"{{.ModulePath}}/{{.SrcImport}}"
)

func main() {
	app := nika.NewApp()

	validator.Setup(app)
	cfg := config.Setup(app, "{{.EnvPath}}")

	transport := natsmq.MustNew(natsmq.Options{
		URL:    cfg.GetString("NATS_URL", "nats://localhost:4222"),
		Prefix: cfg.GetString("NATS_PREFIX", "nika"),
		// Name identifies this connection in `nats server report connections`.
		Name:       cfg.GetString("NATS_NAME", "{{.AppName}}"),
		QueueGroup: cfg.GetString("NATS_QUEUE_GROUP", "{{.AppName}}"),
	})

	if _, err := microservice.Setup(app, microservice.Config{Transport: transport}); err != nil {
		panic(err)
	}

	app.LoadModule(src.NewAppModule())

	// RunWorker starts the consumers, blocks until SIGINT/SIGTERM, then drains
	// in-flight handlers and closes the transport.
	if err := app.RunWorker(); err != nil {
		panic(err)
	}
}
