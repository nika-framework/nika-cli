// Command {{.AppName}} is a Nika microservice served over Redis pub/sub.
//
// Redis pub/sub is at-most-once and stores nothing: a message published while
// this process is restarting is never delivered and the publisher is not told.
// That is the right trade for cache invalidation, presence and live dashboards,
// and the wrong one for anything a business depends on.
package main

import (
	"github.com/nika-framework/nika"
	"github.com/nika-framework/nika/common/config"
	"github.com/nika-framework/nika/common/microservice"
	"github.com/nika-framework/nika/common/microservice/transport/redismq"
	"github.com/nika-framework/nika/common/validator"

	"{{.ModulePath}}/{{.SrcImport}}"
)

func main() {
	app := nika.NewApp()

	validator.Setup(app)
	cfg := config.Setup(app, "{{.EnvPath}}")

	transport := redismq.MustNew(redismq.Options{
		URL: cfg.GetString("REDIS_MQ_URL", "redis://localhost:6379"),
		// Prefix is the only thing keeping two services on one Redis instance
		// from receiving each other's traffic.
		Prefix: cfg.GetString("REDIS_MQ_PREFIX", "nika"),
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
