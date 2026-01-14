package main

import (
	"errors"
	"io"
	"io/fs"
	"net/http"
	"os"
	"os/signal"
	"path"
	"strings"
	"syscall"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/sirrobot01/dbnest/pkg/api"
	"github.com/sirrobot01/dbnest/pkg/config"
	"github.com/sirrobot01/dbnest/pkg/database"
	cruntime "github.com/sirrobot01/dbnest/pkg/runtime"
	"github.com/sirrobot01/dbnest/pkg/scheduler"
	"github.com/sirrobot01/dbnest/pkg/storage"
)

// spaFileServer serves static files with SPA fallback to index.html
// For Vite + React Router, all routing is handled client-side
// We just need to serve index.html for any route that isn't a static file
func spaFileServer(root http.FileSystem) http.Handler {
	fileServer := http.FileServer(root)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Clean the path
		upath := r.URL.Path
		if !strings.HasPrefix(upath, "/") {
			upath = "/" + upath
		}
		upath = path.Clean(upath)

		// Try to open the file
		f, err := root.Open(upath)
		if err == nil {
			stat, _ := f.Stat()
			f.Close()

			// If it's a directory, try index.html inside it
			if stat != nil && stat.IsDir() {
				indexPath := path.Join(upath, "index.html")
				if idx, err := root.Open(indexPath); err == nil {
					idx.Close()
					fileServer.ServeHTTP(w, r)
					return
				}
			} else {
				// It's a file, serve it directly
				fileServer.ServeHTTP(w, r)
				return
			}
		}

		// File doesn't exist - check if it's a route (no extension) or a missing asset
		if !strings.Contains(path.Base(upath), ".") {
			// It's a route like /databases/abc123, serve index.html for client-side routing
			// Serve index.html content directly to avoid redirect loops
			indexFile, err := root.Open("/index.html")
			if err != nil {
				http.NotFound(w, r)
				return
			}
			defer indexFile.Close()

			stat, err := indexFile.Stat()
			if err != nil {
				http.NotFound(w, r)
				return
			}

			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			http.ServeContent(w, r, "index.html", stat.ModTime(), indexFile.(io.ReadSeeker))
			return
		}

		// Missing static asset with extension - return 404
		http.NotFound(w, r)
	})
}

func main() {
	// Create configuration from CLI args
	cfg := config.FromArgs()

	// Setup zerolog
	zerolog.TimeFieldFormat = time.RFC3339
	level, err := zerolog.ParseLevel(string(cfg.LogLevel))
	if err != nil {
		level = zerolog.InfoLevel
	}
	zerolog.SetGlobalLevel(level)
	// Pretty console output for development
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: "15:04:05"})

	// Validate config
	if err := cfg.Validate(); err != nil {
		log.Fatal().Err(err).Msg("Invalid config")
	}

	log.Info().
		Int("port", cfg.Port).
		Str("data_dir", cfg.DataDir).
		Str("runtime", cfg.Runtime).
		Str("socket", cfg.Socket).
		Msg("Starting DBnest")

	// Initialize storage
	store, err := storage.New(cfg.StoragePath(), cfg.DataDir)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to initialize storage")
	}
	defer store.Close()

	// Initialize container runtime client
	runtimeClient, err := cruntime.New(cfg.Runtime, cfg.Socket, cfg.DockerNetwork())
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to initialize container runtime")
	}
	defer func(runtimeClient cruntime.Client) {
		err := runtimeClient.Close()
		if err != nil {
			log.Error().Err(err).Msg("Error closing container runtime client")
		}
	}(runtimeClient)

	// Initialize database manager
	dbManager := database.NewManager(store, runtimeClient)

	// Initialize and start scheduler (handles backups + status sync)
	backupScheduler := scheduler.New(store, dbManager)
	if err := backupScheduler.Start(); err != nil {
		log.Fatal().Err(err).Msg("Failed to start scheduler")
	}

	// Create API server (auth always enabled)
	apiServer := api.NewServer(dbManager, store, runtimeClient)

	// Setup routes
	mux := http.NewServeMux()

	// API routes
	mux.Handle("/api/", apiServer.Handler())

	subFS, err := fs.Sub(frontendContent, "dist")
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to get frontend filesystem")
	}
	log.Info().Msg("Serving embedded frontend")
	frontendHandler := spaFileServer(http.FS(subFS))
	mux.Handle("/", frontendHandler)

	// Start server
	addr := cfg.Addr()
	server := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	// Graceful shutdown
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan

		log.Info().Msg("Shutting down server...")
		backupScheduler.Stop() // Stop scheduler (backups + status sync)
		if err := server.Close(); err != nil {
			log.Error().Err(err).Msg("Error closing server")
		}
	}()

	log.Info().Str("addr", addr).Msg("Server started")
	if err := server.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		log.Fatal().Err(err).Msg("Server error")
	}
}
