// This file assembles the local runtime once and supervises the bounded HTTP
// server lifecycle without creating request-scoped dependency graphs.

package app

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"

	"lina-platform/idc-compliance-gateway/internal/buildinfo"
)

type localWorkerResult struct {
	name string
	err  error
}

// Run serves the operational boundary until cancellation or a server failure.
func Run(ctx context.Context, config Config, info buildinfo.Info) (returnError error) {
	if ctx == nil {
		return fmt.Errorf("context is required")
	}
	runtime, err := buildLocalRuntime(ctx, config.Runtime, config.ReadTimeout)
	if err != nil {
		return err
	}
	defer func() {
		if closeError := runtime.Close(); closeError != nil {
			returnError = errors.Join(
				returnError, fmt.Errorf("close local runtime: %w", closeError),
			)
		}
	}()
	workerContext, cancelWorker := context.WithCancel(ctx)
	defer cancelWorker()
	handler := NewHandler(info, WithLocalRuntime(runtime))
	listenerConfig := net.ListenConfig{}
	listener, err := listenerConfig.Listen(ctx, "tcp", config.ListenAddress)
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}
	server := &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: config.ReadHeaderTimeout,
		ReadTimeout:       config.ReadTimeout,
		WriteTimeout:      config.WriteTimeout,
		IdleTimeout:       config.IdleTimeout,
		MaxHeaderBytes:    config.MaxHeaderBytes,
	}
	serveResult := make(chan error, 1)
	go func() {
		serveResult <- server.Serve(listener)
	}()
	workerResults := make(chan localWorkerResult, 2)
	workerCount := 0
	if runtime.supplierReconciliation.HasQueriers() {
		workerCount++
		go func() {
			workerResults <- localWorkerResult{
				name: "local supplier query worker",
				err:  runtime.supplierReconciliation.RunWorker(workerContext),
			}
		}()
	}
	if runtime.supplierUsageCollection.HasCollectors() {
		workerCount++
		go func() {
			workerResults <- localWorkerResult{
				name: "local supplier usage collection worker",
				err:  runtime.supplierUsageCollection.RunWorker(workerContext),
			}
		}()
	}
	stopWorkers := func(remaining int) error {
		cancelWorker()
		if remaining == 0 {
			return nil
		}
		timer := time.NewTimer(config.ShutdownTimeout)
		defer timer.Stop()
		var result error
		for remaining > 0 {
			select {
			case worker := <-workerResults:
				remaining--
				if worker.err != nil {
					result = errors.Join(result, fmt.Errorf("%s: %w", worker.name, worker.err))
				}
			case <-timer.C:
				return errors.Join(result, fmt.Errorf("local workers did not stop after cancellation"))
			}
		}
		return result
	}
	shutdown := func() error {
		shutdownContext, cancel := context.WithTimeout(context.Background(), config.ShutdownTimeout)
		defer cancel()
		if shutdownError := server.Shutdown(shutdownContext); shutdownError != nil {
			if closeError := server.Close(); closeError != nil {
				return errors.Join(
					fmt.Errorf("shutdown: %w", shutdownError),
					fmt.Errorf("force close after shutdown failure: %w", closeError),
				)
			}
			return fmt.Errorf("shutdown: %w", shutdownError)
		}
		select {
		case serveError := <-serveResult:
			if serveError != nil && !errors.Is(serveError, http.ErrServerClosed) {
				return fmt.Errorf("serve after shutdown: %w", serveError)
			}
			return nil
		case <-time.After(config.ShutdownTimeout):
			if closeError := server.Close(); closeError != nil {
				return errors.Join(
					fmt.Errorf("server did not stop after graceful shutdown"),
					fmt.Errorf("force close timed-out server: %w", closeError),
				)
			}
			return fmt.Errorf("server did not stop after graceful shutdown")
		}
	}

	select {
	case err := <-serveResult:
		workerError := stopWorkers(workerCount)
		if errors.Is(err, http.ErrServerClosed) {
			return workerError
		}
		return errors.Join(fmt.Errorf("serve: %w", err), workerError)
	case worker := <-workerResults:
		cancelWorker()
		shutdownError := shutdown()
		remainingWorkerError := stopWorkers(workerCount - 1)
		if ctx.Err() != nil && worker.err == nil {
			return errors.Join(shutdownError, remainingWorkerError)
		}
		if worker.err == nil {
			worker.err = fmt.Errorf("stopped unexpectedly")
		}
		return errors.Join(
			fmt.Errorf("%s: %w", worker.name, worker.err), shutdownError, remainingWorkerError,
		)
	case <-ctx.Done():
		return errors.Join(shutdown(), stopWorkers(workerCount))
	}
}
