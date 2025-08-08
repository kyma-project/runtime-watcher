package event

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/go-logr/logr"
	"github.com/kyma-project/runtime-watcher/listener/pkg/metrics"
	"github.com/kyma-project/runtime-watcher/listener/pkg/types"
)

const (
	defaultBufferSize      = 100
	defaultReadTimeout     = 30 * time.Second
	defaultWriteTimeout    = 30 * time.Second
	defaultShutdownTimeout = 5 * time.Second
	maxRequestSize         = 1 << 20 // 1MB
)

// Verify is a function which is being called to verify an incoming request to the listener.
// If the verification fails an error should be returned and the request will be dropped,
// otherwise it should return nil.
// If no verification function is needed, a function which just returns nil can be used instead.
type VerifyFunc func(r *http.Request, event *types.WatchEvent) error

// EventListener defines the contract for receiving events.
type EventListener interface {
	// Start begins listening for events and blocks until context is cancelled
	Start(ctx context.Context) error

	// Events returns a read-only channel for consuming events
	Events() <-chan types.WatchEvent

	// Health returns the current health status
	Health() error

	// Stop gracefully shuts down the listener
	Stop(ctx context.Context) error
}

type Config struct {
	Addr          string
	ComponentName string
	VerifyFunc    VerifyFunc
	Logger        logr.Logger
}

type HTTPEventListener struct {
	config    Config
	server    *http.Server
	events    chan types.WatchEvent
	healthErr error
	healthMux sync.RWMutex
	logger    logr.Logger
	stopped   bool
	stopMux   sync.Mutex
}

func NewHTTPEventListener(config Config) *HTTPEventListener {
	if config.Logger.GetSink() == nil {
		config.Logger = logr.Discard()
	}

	listener := &HTTPEventListener{
		config: config,
		events: make(chan types.WatchEvent, defaultBufferSize),
		logger: config.Logger.WithName("http-event-listener"),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/health", listener.handleHealth)
	mux.HandleFunc("/v1/"+config.ComponentName+"/event", listener.handleEvent)

	listener.server = &http.Server{
		Addr:         config.Addr,
		Handler:      mux,
		ReadTimeout:  defaultReadTimeout,
		WriteTimeout: defaultWriteTimeout,
	}

	return listener
}

func (l *HTTPEventListener) Start(ctx context.Context) error {
	l.logger.Info("starting HTTP event listener", "addr", l.config.Addr)

	errChan := make(chan error, 1)

	go func() {
		err := l.server.ListenAndServe()
		if err != nil && err != http.ErrServerClosed {
			errChan <- err
		}
	}()

	select {
	case err := <-errChan:
		l.setHealth(err)
		return err
	case <-ctx.Done():
		l.logger.Info("HTTP events listener shutting down")
		return fmt.Errorf("HTTP listener context cancelled: %w", ctx.Err())
	}
}

func (l *HTTPEventListener) Events() <-chan types.WatchEvent {
	return l.events
}

func (l *HTTPEventListener) Health() error {
	l.healthMux.RLock()
	defer l.healthMux.RUnlock()

	return l.healthErr
}

func (l *HTTPEventListener) Stop(ctx context.Context) error {
	l.stopMux.Lock()
	defer l.stopMux.Unlock()

	if l.stopped {
		return nil // Already stopped
	}

	shutdownCtx, cancel := context.WithTimeout(ctx, defaultShutdownTimeout)
	defer cancel()

	err := l.server.Shutdown(shutdownCtx)
	if err != nil {
		l.logger.Error(err, "HTTP server shutdown failed")
		return fmt.Errorf("failed to shutdown HTTP server: %w", err)
	}

	close(l.events)
	l.stopped = true

	return nil
}

func (l *HTTPEventListener) handleHealth(writer http.ResponseWriter, request *http.Request) {
	err := l.Health()
	if err != nil {
		http.Error(writer, err.Error(), http.StatusServiceUnavailable)
		return
	}

	writer.WriteHeader(http.StatusOK)
	_, _ = writer.Write([]byte("OK"))
}

func (l *HTTPEventListener) handleEvent(writer http.ResponseWriter, request *http.Request) {
	startTime := time.Now()

	metrics.RecordHTTPInflightRequests(1)

	defer func() {
		metrics.RecordHTTPInflightRequests(-1)
		metrics.UpdateHTTPRequestMetrics(time.Since(startTime))
	}()

	if request.Method != http.MethodPost {
		metrics.RecordHTTPRequestErrors()
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)

		return
	}

	// Check request size limit
	if request.ContentLength > maxRequestSize {
		metrics.RecordHTTPRequestExceedingSizeLimit()
		http.Error(writer, "request too large", http.StatusRequestEntityTooLarge)

		return
	}

	event, unmarshalErr := UnmarshalSKREvent(request)
	if unmarshalErr != nil {
		l.logger.Error(nil, "event unmarshal failed", "error", unmarshalErr.Message)
		metrics.RecordHTTPRequestErrors()
		http.Error(writer, unmarshalErr.Message, unmarshalErr.HTTPErrorCode)

		return
	}

	if l.config.VerifyFunc != nil {
		err := l.config.VerifyFunc(request, event)
		if err != nil {
			l.logger.Error(err, "event verification failed")
			metrics.RecordHTTPFailedVerificationRequests(request.RequestURI)
			http.Error(writer, "verification failed", http.StatusUnauthorized)

			return
		}
	}

	select {
	case l.events <- *event:
		writer.WriteHeader(http.StatusOK)
	default:
		// Check if we're shutting down while holding the lock to prevent races
		l.stopMux.Lock()
		stopped := l.stopped
		l.stopMux.Unlock()

		if stopped {
			http.Error(writer, "listener is shutting down", http.StatusServiceUnavailable)
		} else {
			l.logger.Error(nil, "event channel full, dropping event", "owner", event.Owner)
			metrics.RecordHTTPRequestErrors()
			http.Error(writer, "event queue full", http.StatusServiceUnavailable)
		}
	}
}

func (l *HTTPEventListener) setHealth(err error) {
	l.healthMux.Lock()
	defer l.healthMux.Unlock()

	l.healthErr = err
}
