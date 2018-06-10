package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"syscall"

	"github.com/DMarby/picsum-photos/api"
	"github.com/DMarby/picsum-photos/image"
	"github.com/DMarby/picsum-photos/queue"
	"github.com/oklog/run"
)

func handleInterrupt(ctx context.Context) error {
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
	select {
	case sig := <-c:
		return fmt.Errorf("received signal %s", sig)
	case <-ctx.Done():
		return errors.New("canceled")
	}
}

func getWorkerCount() int {
	workers := runtime.NumCPU() - 1

	if workers < 1 {
		workers = 1
	}

	return workers
}

func main() {
	var g run.Group

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start worker queue
	workerQueue := queue.New(ctx, getWorkerCount(), func(data interface{}) (interface{}, error) {
		stringData := data.(string)
		return stringData, nil
	})

	g.Add(func() error {
		workerQueue.Run()
		return nil
	}, func(error) {
		cancel()
	})

	// Get imageProcessor instance
	imageProcessor, err := image.GetInstance()
	if err != nil {
		cancel() // TODO: Verify that this works as expected
		return
	}

	// Start and listen on http
	api := api.New(workerQueue, imageProcessor)
	server := &http.Server{
		Addr:    ":8080",
		Handler: api.Router(),
	}

	g.Add(func() error {
		return server.ListenAndServe()
	}, func(error) {
		server.Shutdown(ctx)
	})

	// Exit if we receieve SIGINT or SIGTERM
	g.Add(func() error {
		return handleInterrupt(ctx)
	}, func(error) {
		imageProcessor.Shutdown()
	})

	log.Print(g.Run().Error())
}
