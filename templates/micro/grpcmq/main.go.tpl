// Command {{.AppName}} is a Nika microservice served over gRPC.
//
// gRPC is synchronous RPC, not a broker: there is no store-and-forward, no
// fan-out, and a Publish still costs a full round trip. Use it for
// service-to-service calls where a caller is waiting and a failure should be
// visible immediately.
//
// No protoc step is involved — the service is a hand-written grpc.ServiceDesc
// with a pass-through codec, so there is no .proto file to generate from.
package main

import (
	"github.com/nika-framework/nika"
	"github.com/nika-framework/nika/common/config"
	"github.com/nika-framework/nika/common/microservice"
	"github.com/nika-framework/nika/common/microservice/transport/grpcmq"
	"github.com/nika-framework/nika/common/validator"

	"{{.ModulePath}}/{{.SrcImport}}"
)

func main() {
	app := nika.NewApp()

	validator.Setup(app)
	cfg := config.Setup(app, "{{.EnvPath}}")

	transport := grpcmq.MustNew(grpcmq.Options{
		// Addr is the server half; Target is this process's own client half,
		// used when it calls out over gRPC.
		Addr:   cfg.GetString("GRPC_ADDR", ":50051"),
		Target: cfg.GetString("GRPC_TARGET", ""),

		// Plaintext must be opted into explicitly — grpcmq has no implicit
		// insecure default. Fine behind a service mesh or on localhost; set
		// GRPC_INSECURE=false and supply TLSConfig before this crosses a
		// network boundary, because envelopes carry auth headers in clear.
		Insecure: cfg.GetBool("GRPC_INSECURE", true),
	})

	if _, err := microservice.Setup(app, microservice.Config{Transport: transport}); err != nil {
		panic(err)
	}

	app.LoadModule(src.NewAppModule())

	// RunWorker starts the gRPC server, blocks until SIGINT/SIGTERM, then
	// drains in-flight calls and stops it.
	if err := app.RunWorker(); err != nil {
		panic(err)
	}
}
