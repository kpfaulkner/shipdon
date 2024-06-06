package main

import (
	"context"
	"flag"
	"fmt"
	"gioui.org/app"
	"gioui.org/unit"
	"git.sr.ht/~gioverse/skel/stream"
	config2 "github.com/kpfaulkner/shipdon/config"
	"github.com/kpfaulkner/shipdon/events"
	"github.com/kpfaulkner/shipdon/mastodon"
	"github.com/kpfaulkner/shipdon/ui"
	log "github.com/sirupsen/logrus"
	"net/http"
	_ "net/http/pprof"
	"os"
	"runtime"
	"time"
)

var (
	AccountID int64
	Version   = "0.1.2"
)

func setupLogging(debug bool) {
	log.SetFormatter(&log.JSONFormatter{})

	file, err := os.OpenFile("shipdon.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err == nil {
		log.SetOutput(file)
	} else {
		log.SetOutput(os.Stdout)
		log.Info("Failed to log to file, using default stderr")
	}
	log.SetFormatter(&log.TextFormatter{TimestampFormat: "2006-01-02 15:04:05", FullTimestamp: true})

	if debug {
		log.SetLevel(log.DebugLevel)
	} else {
		log.SetLevel(log.InfoLevel)
	}
}

func PrintMemUsage() {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	// For info on each, see: https://golang.org/pkg/runtime/#MemStats
	fmt.Printf("Alloc = %v MiB", bToMb(m.Alloc))
	fmt.Printf("\tTotalAlloc = %v MiB", bToMb(m.TotalAlloc))
	fmt.Printf("\tSys = %v MiB", bToMb(m.Sys))
	fmt.Printf("\tNumGC = %v", m.NumGC)
	fmt.Printf("\tHeapObjects = %v", m.HeapObjects)
	fmt.Printf("\tHeapReleased = %v\n", m.HeapReleased)
}

func bToMb(b uint64) uint64 {
	return b / 1024 / 1024
}

func main() {

	debug := flag.Bool("debug", false, "enable debug mode")
	enablePprof := flag.Bool("pprof", false, "enable pprof. listen on port 6060")
	enableMemStats := flag.Bool("mem", false, "print memory stats on stdout every minute")
	flag.Parse()

	if *enablePprof {
		go func() {
			log.Println(http.ListenAndServe("localhost:6060", nil))
		}()
	}

	if *enableMemStats {
		go func() {
			for {
				PrintMemUsage()
				time.Sleep(1 * time.Minute)
			}
		}()

	}

	setupLogging(*debug)
	config := config2.LoadConfig()
	appCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	w := new(app.Window)
	opts := []app.Option{}

	opts = append(opts, app.Size(unit.Dp(800), unit.Dp(800)))
	opts = append(opts, app.Title("Shipdon : "+Version))
	w.Option(opts...)

	windowCtx, cancel := context.WithCancel(appCtx)
	controller := stream.NewController(windowCtx, w.Invalidate)

	// using global event channel.... need to refactor
	eventListener := events.NewEventLister(events.EventChannel)

	// launches a go routine for listening.
	eventListener.Listen()

	backend, err := mastodon.NewMastodonBackend(eventListener, config)
	if err != nil {
		log.Fatalf("could not create mastodon client: %v", err)
	}

	th := ui.GenerateDarkTheme()

	uinterface := ui.NewUI(
		controller,
		w,
		ui.NewComposeColumn(ui.NewComponentState(controller, backend), th),
		[]*ui.MessageColumn{},
		backend,
		eventListener,
		config,
	)

	go func() {
		if err := uinterface.Run(); err != nil {
			log.Fatal(err)
		}
		os.Exit(0)
	}()
	app.Main()
}
