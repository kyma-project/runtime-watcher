package event

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/kyma-project/runtime-watcher/listener/pkg/types"

	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/event"
	ctrlLog "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const paramContractVersion = "1"

func RegisterListenerComponent(addr, componentName string, verify Verify) (*SKREventListener, *source.Channel) {
	eventSource := make(chan event.GenericEvent)
	return &SKREventListener{
		Addr:           addr,
		ComponentName:  componentName,
		receivedEvents: eventSource,
		VerifyFunc:     verify,
	}, &source.Channel{Source: eventSource}
}

// Verify is a function which is being called to verify an incomming request to the listener.
// If the verification fails an error should be returned and the request will be dropped,
// otherwise it should return nil.
// If no verification function is needed, a function which just returns nil can be used instead.
type Verify func(r *http.Request, watcherEvtObject *types.WatchEvent) error

type SKREventListener struct {
	Addr           string
	Logger         logr.Logger
	ComponentName  string
	receivedEvents chan event.GenericEvent
	VerifyFunc     Verify
}

func (l *SKREventListener) GetReceivedEvents() chan event.GenericEvent {
	if l.receivedEvents == nil {
		l.receivedEvents = make(chan event.GenericEvent)
	}
	return l.receivedEvents
}

func (l *SKREventListener) Start(ctx context.Context) error {
	l.Logger = ctrlLog.FromContext(ctx, "Module", "Listener")

	router := http.NewServeMux()

	listenerPattern := fmt.Sprintf("/v%s/%s/event", paramContractVersion, l.ComponentName)

	router.HandleFunc(listenerPattern, l.HandleSKREvent())

	// start web server
	const defaultTimeout = time.Second * 60
	server := &http.Server{
		Addr: l.Addr, Handler: router,
		ReadHeaderTimeout: defaultTimeout, ReadTimeout: defaultTimeout,
		WriteTimeout: defaultTimeout,
	}
	go func() {
		l.Logger.WithValues(
			"Addr", l.Addr,
			"ApiPath", listenerPattern,
		).Info("Listener is starting up...")
		err := server.ListenAndServe()
		if err != nil && err != http.ErrServerClosed {
			l.Logger.Error(err, "Webserver startup failed")
		}
	}()
	<-ctx.Done()
	l.Logger.Info("SKR events listener is shutting down: context got closed")
	return server.Shutdown(ctx)
}

func (l *SKREventListener) HandleSKREvent() http.HandlerFunc {
	return func(writer http.ResponseWriter, req *http.Request) {
		// http method support: POST only is allowed
		if req.Method != http.MethodPost {
			errorMessage := fmt.Sprintf("%s method is not allowed on this path", req.Method)
			l.Logger.Error(nil, errorMessage)
			http.Error(writer, errorMessage, http.StatusMethodNotAllowed)
			return
		}

		l.Logger.V(1).Info("received event from SKR")

		// unmarshal received event
		watcherEvent, unmarshalErr := UnmarshalSKREvent(req)
		if unmarshalErr != nil {
			l.Logger.Error(nil, unmarshalErr.Message)
			http.Error(writer, unmarshalErr.Message, unmarshalErr.HTTPErrorCode)
			return
		}

		// verify request
		if err := l.VerifyFunc(req, watcherEvent); err != nil {
			l.Logger.Info("request could not be verified - Event will not be dispatched",
				"error", err)
			return
		}

		genericEvtObject := GenericEvent(watcherEvent)
		// add event to the channel
		l.receivedEvents <- event.GenericEvent{Object: genericEvtObject}
		l.Logger.Info("dispatched event object into channel", "resource-name", genericEvtObject.GetName())
		writer.WriteHeader(http.StatusOK)
	}
}
