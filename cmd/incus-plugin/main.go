package main

import (
	"context"
	"fmt"
	"os"

	"github.com/CUTe-CCNL/Capsule/internal/incusplugin"
	"github.com/CUTe-CCNL/host-agent/pkg/pluginipc"
)

func main() {
	config, err := incusplugin.ConfigFromEnv()
	if err != nil {
		exitError(err)
	}

	client, err := incusplugin.NewIncusClient(config)
	if err != nil {
		exitError(err)
	}

	handler := incusplugin.NewHandler(config, client)
	if err := pluginipc.Serve(context.Background(), handler); err != nil {
		exitError(err)
	}
}

func exitError(err error) {
	_, _ = fmt.Fprintf(os.Stderr, "incus-plugin: %v\n", err)
	os.Exit(1)
}
