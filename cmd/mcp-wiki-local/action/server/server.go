package server

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/tyrm/mcp-wikipedia-local/cmd/pupjournal/action"
	"github.com/tyrm/mcp-wikipedia-local/internal/logic"
	"go.uber.org/zap"
)

var Start action.Action = func(ctx context.Context, args []string) error {
	ctx, cancel := context.WithCancel(ctx)

	logicMod := logic.New(&logic.Config{
		DB:   dbClient,
		HTTP: httpClient,
	})

	// ** start application **
	errChan := make(chan error)

	// Wait for SIGINT and SIGTERM (HIT CTRL-C)
	stopSigChan := make(chan os.Signal, 1)
	signal.Notify(stopSigChan, syscall.SIGINT, syscall.SIGTERM)

	// start webserver
	//go func(s *http.Server, errChan chan error) {
	//	zap.L().Info("starting http server")
	//	err := s.Start()
	//	if err != nil {
	//		errChan <- fmt.Errorf("http server: %s", err.Error())
	//	}
	//}(httpServer, errChan)

	// wait for event
	select {
	case sig := <-stopSigChan:
		zap.L().Info("got signal", zap.String("signal", sig.String()))
	case err := <-errChan:
		zap.L().Fatal("fatal error", zap.Error(err))
	}

	zap.L().Info("done")
	cancel()

	return nil
}
