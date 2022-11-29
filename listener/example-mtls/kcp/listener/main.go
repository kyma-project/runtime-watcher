package main

import (
	"context"
	"fmt"
	"net/http"

	"github.com/kyma-project/runtime-watcher/listener/pkg/event"
	"github.com/kyma-project/runtime-watcher/listener/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

func main() {
	ctx := context.Background()
	logger := ctrl.Log.WithName("example-listener")
	logf.SetLogger(zap.New(zap.UseFlagOptions(&zap.Options{
		Development: true,
	})))
	skrEvent, _ := event.RegisterListenerComponent(":8089", "example-listener",
		func(r *http.Request, watcherEvtObject *types.WatchEvent) error {
			return nil
		})

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
