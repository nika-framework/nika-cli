// Command {{.AppName}} is a Nika microservice served over AMQP 0-9-1 (RabbitMQ).
//
// RABBITMQ_QUEUE must be distinct per service: two services sharing one queue
// compete for messages instead of both receiving them. It is a durable, named
// queue on purpose — an anonymous auto-delete queue drops every message
// published while the service is down, silently.
package main

import (
	"github.com/nika-framework/nika"
	"github.com/nika-framework/nika/common/config"
	"github.com/nika-framework/nika/common/microservice"
	"github.com/nika-framework/nika/common/microservice/transport/rabbitmq"
	"github.com/nika-framework/nika/common/validator"

	"{{.ModulePath}}/{{.SrcImport}}"
)

func main() {
	app := nika.NewApp()

	validator.Setup(app)
	cfg := config.Setup(app, "{{.EnvPath}}")

	transport := rabbitmq.MustNew(rabbitmq.Options{
		URL:      cfg.GetString("RABBITMQ_URL", "amqp://guest:guest@localhost:5672/"),
		Exchange: cfg.GetString("RABBITMQ_EXCHANGE", "nika"),
		Queue:    cfg.GetString("RABBITMQ_QUEUE", "nika.{{.AppName}}"),
		Prefetch: cfg.GetInt("RABBITMQ_PREFETCH", 32),
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
