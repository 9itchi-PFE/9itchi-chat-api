package app

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"9itchi-chat-api/repositories"
	"9itchi-chat-api/router"
	"9itchi-chat-api/services"
	"9itchi-chat-api/utils"

	"github.com/gin-gonic/gin"
)

type App struct {
	Config   utils.AppConfig
	Router   *gin.Engine
	Repos    *repositories.Repositories
	Services *services.Services
}

func New() *App {
	app := &App{}
	app.setup()
	return app
}

// setup order: config -> repositories (connect + migrate) -> services -> router.
func (app *App) setup() {
	config := utils.LoadConfig()
	if err := config.Validate(); err != nil {
		log.Fatalf("invalid configuration: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	repos, err := repositories.InitRepositories(ctx, config)
	if err != nil {
		log.Fatalf("repositories: %v", err)
	}

	servicesWrapper := services.InitServices(config, repos)

	app.Config = config
	app.Repos = repos
	app.Services = servicesWrapper
	app.Router = router.InitializeRouter(servicesWrapper)
}

// Run serves until SIGINT/SIGTERM, then drains requests and closes the database pool.
func (app *App) Run() {
	srv := &http.Server{
		Addr:              fmt.Sprintf(":%d", app.Config.Port),
		Handler:           app.Router,
		ReadHeaderTimeout: 10 * time.Second,
	}
	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server: %v", err)
		}
	}()

	sweep, stopSweep := context.WithCancel(context.Background())
	go app.Services.ReplyNotifier.Run(sweep)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	stopSweep()
	_ = srv.Shutdown(ctx)
	app.Repos.Close()
}
