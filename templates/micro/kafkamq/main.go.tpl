// Command {{.AppName}} is a Nika microservice served over Apache Kafka.
//
// Every message travels on one topic with the pattern as the Kafka key, so all
// messages for a subject land on the same partition and stay ordered. KAFKA_GROUP_ID
// is required: without a consumer group every replica reads every partition, so
// the service is fanned out rather than load-balanced.
package main

import (
	"strings"

	"github.com/nika-framework/nika"
	"github.com/nika-framework/nika/common/config"
	"github.com/nika-framework/nika/common/microservice"
	"github.com/nika-framework/nika/common/microservice/transport/kafkamq"
	"github.com/nika-framework/nika/common/validator"

	"{{.ModulePath}}/{{.SrcImport}}"
)

func main() {
	app := nika.NewApp()

	validator.Setup(app)
	cfg := config.Setup(app, "{{.EnvPath}}")

	transport := kafkamq.MustNew(kafkamq.Options{
		Brokers: splitList(cfg.GetString("KAFKA_BROKERS", "localhost:9092")),
		Topic:   cfg.GetString("KAFKA_TOPIC", "nika"),
		GroupID: cfg.GetString("KAFKA_GROUP_ID", "{{.AppName}}"),
		// Concurrency defaults to 1 because concurrency and per-partition
		// ordering are mutually exclusive. Raise it only when order does not
		// matter for this service.
		Concurrency: cfg.GetInt("KAFKA_CONCURRENCY", 1),
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

// splitList turns "a:9092,b:9092" into a broker slice, dropping blanks so a
// trailing comma in .env is not read as an empty broker address.
func splitList(value string) []string {
	var out []string
	for _, part := range strings.Split(value, ",") {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}
