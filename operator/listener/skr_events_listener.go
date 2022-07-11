package listener

import (
	"context"
	"fmt"
	"net/http"

	"github.com/go-logr/logr"
	"github.com/go-logr/zapr"
	"go.uber.org/zap"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const paramContractVersion = "contractVersion"

func RegisterListenerComponent(addr, componentName string) (*SKREventsListener, *source.Channel) {

	var log logr.Logger
	zapLog, err := zap.NewDevelopment()
	if err != nil {
		panic(fmt.Sprintf("failed to initialize zap logger: %v?", err))
	}
	log = zapr.NewLogger(zapLog)
	eventSource := make(chan event.GenericEvent)
	return &SKREventsListener{
		addr:           addr,
		logger:         log,
		componentName:  componentName,
		receivedEvents: eventSource,
	}, &source.Channel{Source: eventSource}

}

type SKREventsListener struct {
	addr           string
	logger         logr.Logger
	componentName  string
	receivedEvents chan event.GenericEvent
}

func (l *SKREventsListener) ReceivedEvents() chan event.GenericEvent {
	if l.receivedEvents == nil {
		l.receivedEvents = make(chan event.GenericEvent)
	}
	return l.receivedEvents
}

func (l *SKREventsListener) Start(ctx context.Context) error {

	router := http.NewServeMux()

	router.HandleFunc(
		fmt.Sprintf("/v%s/%s/event", paramContractVersion, l.componentName),
		l.handleSKREvent(),
	)

	//start web server
	server := &http.Server{Addr: l.addr, Handler: router}
	go func() {
		l.logger.Info("SKR events listener is starting up...")
		err := server.ListenAndServe()
		if err != nil && err != http.ErrServerClosed {
			l.logger.Error(err, "Webserver startup failed")
		}
	}()
	<-ctx.Done()
	l.logger.Info("SKR events listener is shutting down: context got closed")
	return server.Shutdown(ctx)

}

func (l *SKREventsListener) handleSKREvent() http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {

		//http method support: POST only is allowed
		if r.Method != http.MethodPost {
			errorMessage := fmt.Sprintf("%s method is not allowed on this path", r.Method)
			l.logger.Error(nil, errorMessage)
			http.Error(w, errorMessage, http.StatusMethodNotAllowed)
			return
		}

		l.logger.Info("received event from SKR")

		//unmarshal received event
		genericEvtObject, unmarshalErr := unmarshalSKREvent(r)
		if unmarshalErr != nil {
			l.logger.Error(nil, unmarshalErr.Message)
			http.Error(w, unmarshalErr.Message, unmarshalErr.httpErrorCode)
			return
		}

		//add event to the channel
		l.receivedEvents <- event.GenericEvent{Object: genericEvtObject}
		l.logger.Info("dispatched event object into channel", "resource-name", genericEvtObject.GetName())
		w.WriteHeader(http.StatusOK)

	}

}
