package event

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/kyma-project/runtime-watcher/listener/pkg/metrics"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	paramContractVersion = "1"
)

type SKREventListener struct {
	httpListener   *HTTPEventListener
	eventChannel   chan ControllerRuntimeEvent
	ReceivedEvents <-chan ControllerRuntimeEvent
	logger         logr.Logger
	Addr           string
	ComponentName  string
	VerifyFunc     VerifyFunc
}

// ControllerRuntimeEvent represents the controller-runtime GenericEvent structure.
type ControllerRuntimeEvent struct {
	Object *unstructured.Unstructured
}

func NewSKREventListener(addr, componentName string, verifyFunc VerifyFunc) *SKREventListener {
	eventChannel := make(chan ControllerRuntimeEvent, defaultBufferSize)

	config := Config{
		Addr:          addr,
		ComponentName: componentName,
		VerifyFunc:    verifyFunc,
		Logger:        logr.Discard(),
	}

	httpListener := NewHTTPEventListener(config)

	return &SKREventListener{
		httpListener:   httpListener,
		eventChannel:   eventChannel,
		ReceivedEvents: eventChannel,
		Addr:           addr,
		ComponentName:  componentName,
		VerifyFunc:     verifyFunc,
	}
}

func (s *SKREventListener) Start(ctx context.Context) error {
	if logger := logr.FromContextOrDiscard(ctx); logger.GetSink() != nil {
		s.logger = logger.WithValues("Module", "Listener")
	} else {
		s.logger = logr.Discard().WithValues("Module", "Listener")
	}

	listenerPattern := fmt.Sprintf("/v%s/%s/event", paramContractVersion, s.ComponentName)

	s.logger.Info("Listener starting", "addr", s.Addr, "path", listenerPattern)

	s.httpListener.config.Logger = s.logger.WithName("http-event-listener")
	s.httpListener.logger = s.httpListener.config.Logger

	// Start the HTTP listener
	errChan := make(chan error, 1)

	go func() {
		err := s.httpListener.Start(ctx)
		if err != nil {
			s.logger.Error(err, "HTTP listener failed")

			errChan <- err
		}
	}()

	go s.convertEvents(ctx)

	select {
	case err := <-errChan:
		return err
	case <-ctx.Done():
		s.logger.Info("SKR events listener is shutting down")
		return s.httpListener.Stop(context.Background())
	}
}

func (s *SKREventListener) convertEvents(ctx context.Context) {
	defer func() {
		close(s.eventChannel)
	}()

	for {
		select {
		case event, ok := <-s.httpListener.Events():
			if !ok {
				return
			}

			genericEvtObject := GenericEvent(&event)

			s.logger.Info("Event processed", "owner", event.Owner.String())

			select {
			case s.eventChannel <- ControllerRuntimeEvent{Object: genericEvtObject}:
				metrics.RecordEventProcessed(event.Owner.String())
			case <-ctx.Done():
				s.logger.Info("context cancelled during event forwarding")
				return
			default:
				s.logger.Error(nil, "event channel full, dropping event",
					"owner", event.Owner.String(),
					"channelCapacity", cap(s.eventChannel))
				metrics.RecordEventDropped("channel_full")
			}
		case <-ctx.Done():
			return
		}
	}
}
