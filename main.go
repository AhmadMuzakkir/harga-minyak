package main

import (
	"context"
	"github.com/bububa/cron"
	"github.com/getsentry/raven-go"
	"github.com/kelseyhightower/envconfig"
	"github.com/ahmadmuzakkir/harga-minyak/api"
	"github.com/ahmadmuzakkir/harga-minyak/provider"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"
	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"github.com/go-chi/render"
)

var authorized map[string]struct{}
var env Env

type Env struct {
	SentryDsn string   `envconfig:"SENTRY_DSN"`
	Port      int      `envconfig:"PORT"`
	ApiKeys   []string `envconfig:"API_KEYS"`
}

func main() {
	env = Env{}
	envconfig.MustProcess("", &env)

	if env.Port == 0 {
		panic("Port cannot be empty")
		return
	}

	log.Println("Port: ", env.Port)
	log.Println("Raven: ", env.SentryDsn)

	if env.SentryDsn != "" {
		raven.SetDSN(env.SentryDsn)
	}

	setAuthorization(env.ApiKeys)

	r := chi.NewRouter()

	mysumberProvider := provider.NewClient()
	hargaHandler := api.NewHandler(mysumberProvider)

	r.Use(middleware.Logger)
	r.Use(Recoverer)
	r.Use(middleware.DefaultCompress)
	r.Use(render.SetContentType(render.ContentTypeJSON))
	r.Use(Authorization)
	r.Mount("/", hargaHandler.Routes())

	// Init and run the cron
	var mycron *cron.Cron
	raven.CapturePanicAndWait(func() {
		mycron = startCron(hargaHandler)
	}, map[string]string{"module": "cron"})

	// Init the cache
	hargaHandler.RefreshCache()

	// Init the Http Server
	httpServer := &http.Server{Addr: ":" + strconv.Itoa(env.Port), Handler: r}

	go func() {
		// Run the Http server
		log.Fatal(httpServer.ListenAndServe())
	}()

	// Block until the termination signal is sent
	shutdownSignal := make(chan os.Signal, 1)
	signal.Notify(shutdownSignal, os.Interrupt, syscall.SIGTERM)
	<-shutdownSignal

	if mycron != nil {
		mycron.Stop()
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	err := httpServer.Shutdown(ctx)
	if err != nil {
		log.Println("Error shutting down HTTP server, ", err)
	}
}

func startCron(handler *api.HargaHandler) *cron.Cron {
	loc, err := time.LoadLocation("Asia/Kuala_Lumpur")
	if err != nil {
		log.Panic(err)
	}
	c := cron.NewWithLocation(loc)

	c.AddFunc("no1", "0 0 8 * * *", func() {
		handler.RefreshCache()
	})

	c.AddFunc("no2", "0 0 20 * * *", func() {
		handler.RefreshCache()
	})

	c.Start()

	return c
}

func Recoverer(next http.Handler) http.Handler {
	fn := raven.RecoveryHandler(next.ServeHTTP)

	return http.HandlerFunc(fn)
}

func setAuthorization(apiKeys []string) {
	if apiKeys == nil {
		authorized = nil
		return
	}

	authorized = make(map[string]struct{})
	for _, v := range apiKeys {
		authorized[v] = struct{}{}
	}
}

func Authorization(next http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")

		if authorized == nil {
			next.ServeHTTP(w, r)
			return
		}

		_, exist := authorized[auth]
			if auth != "" && exist {
				next.ServeHTTP(w, r)
				return
		}

		w.WriteHeader(http.StatusUnauthorized)
	}

	return http.HandlerFunc(fn)
}
