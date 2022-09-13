package main

import (
	"context"
	"fmt"
	"github.com/kyma-project/runtime-watcher/listener/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/log"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

func main() {
	ctx := context.Background()
	logger := log.Log.WithName("example-listener")
	opts := zap.Options{
		Development: true,
	}
	logf.SetLogger(zap.New(zap.UseFlagOptions(&opts)))
	skrEvent, _ := event.RegisterListenerComponent(":8089", "example-listener")
	go func() {
		for {
			select {
			case response := <-skrEvent.GetReceivedEvents():
				logger.Info("watcher event received....")
				logger.Info(fmt.Sprintf("%v", response.Object))
			case <-ctx.Done():
				logger.Info("context closed")
				return
			}
		}
	}()

	if err := skrEvent.Start(ctx); err != nil {
		logger.Error(err, "cannot start listener")
	}
}
