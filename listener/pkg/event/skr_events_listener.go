package event

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/kyma-project/runtime-watcher/listener/pkg/metrics"
	"sigs.k8s.io/controller-runtime/pkg/event"
	ctrlLog "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/go-logr/logr"
	"github.com/kyma-project/runtime-watcher/listener/pkg/types"
)

const (
	paramContractVersion    = "1"
	requestSizeLimitInBytes = 16384 // 16KB
)

// Verify is a function which is being called to verify an incoming request to the listener.
// If the verification fails an error should be returned and the request will be dropped,
// otherwise it should return nil.
// If no verification function is needed, a function which just returns nil can be used instead.
type Verify func(r *http.Request, watcherEvtObject *types.WatchEvent) error

type SKREventListener struct {
	Addr           string
	Logger         logr.Logger
	ComponentName  string
	ReceivedEvents chan event.GenericEvent
	VerifyFunc     Verify
}

func NewSKREventListener(addr, componentName string, verify Verify,
) *SKREventListener {
	return &SKREventListener{
		Addr:           addr,
		ComponentName:  componentName,
		ReceivedEvents: make(chan event.GenericEvent),
		VerifyFunc:     verify,
	}
}

func (l *SKREventListener) Start(ctx context.Context) error {
	l.Logger = ctrlLog.FromContext(ctx, "Module", "Listener")
	router := http.NewServeMux()

	listenerPattern := fmt.Sprintf("/v%s/%s/event", paramContractVersion, l.ComponentName)
	router.HandleFunc(listenerPattern, l.RequestSizeLimitingMiddleware(l.HandleSKREvent()))

	// start web server
	const defaultTimeout = time.Second * 60
	server := &http.Server{
		Addr: l.Addr, Handler: http.MaxBytesHandler(router, requestSizeLimitInBytes),
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
	err := server.Shutdown(ctx)
	if err != nil {
		return fmt.Errorf("failed to shutdown server: %w", err)
	}
	return nil
}

func (l *SKREventListener) RequestSizeLimitingMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		if request.ContentLength > requestSizeLimitInBytes {
			metrics.RecordHTTPRequestExceedingSizeLimit()
			errorMessage := fmt.Sprintf("Body size greater than %d bytes is not allowed", requestSizeLimitInBytes)
			l.Logger.Error(errors.New("requestSizeExceeded"), errorMessage)
			http.Error(writer, errorMessage, http.StatusRequestEntityTooLarge)
			return
		}

		executeRequestAndUpdateMetrics(next, writer, request)
	}
}

func (l *SKREventListener) HandleSKREvent() http.HandlerFunc {
	return func(writer http.ResponseWriter, req *http.Request) {
		// http method support: POST only is allowed
		if req.Method != http.MethodPost {
			errorMessage := req.Method + " method is not allowed on this path"
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
			metrics.RecordHTTPFailedVerificationRequests(req.RequestURI)
			l.Logger.Info("request could not be verified - Event will not be dispatched",
				"error", err)
			return
		}

		genericEvtObject := GenericEvent(watcherEvent)
		// add event to the channel
		l.ReceivedEvents <- event.GenericEvent{Object: genericEvtObject}
		l.Logger.Info("dispatched event object into channel", "resource-name", genericEvtObject.GetName())
		writer.WriteHeader(http.StatusOK)
	}
}

func executeRequestAndUpdateMetrics(next http.HandlerFunc, writer http.ResponseWriter, request *http.Request) {
	start := time.Now()
	next.ServeHTTP(writer, request)

	duration := time.Since(start)
	metrics.UpdateHTTPRequestMetrics(duration)
	metrics.RecordHTTPInflightRequests(1)

	if request.Response != nil && request.Response.Status != strconv.Itoa(http.StatusOK) {
		metrics.RecordHTTPRequestErrors()
	}

	defer metrics.RecordHTTPInflightRequests(-1)
}
