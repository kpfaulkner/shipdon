package main

import (
	"context"
	"gioui.org/app"
	"gioui.org/unit"
	"git.sr.ht/~gioverse/skel/stream"
	config2 "github.com/kpfaulkner/shipdon/config"
	"github.com/kpfaulkner/shipdon/events"
	"github.com/kpfaulkner/shipdon/mastodon"
	"github.com/kpfaulkner/shipdon/ui"
	log "github.com/sirupsen/logrus"
	"os"
)

var (
	AccountID int64
)

func main() {

	log.SetFormatter(&log.JSONFormatter{})
	log.SetOutput(os.Stdout)
	log.SetLevel(log.DebugLevel)

	config := config2.LoadConfig()
	appCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	w := new(app.Window)
	opts := []app.Option{}

	opts = append(opts, app.Size(unit.Dp(800), unit.Dp(800)))
	opts = append(opts, app.Title("Shipdon"))
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
		[]*ui.MessageColumn{
			ui.NewMessageColumn(ui.NewComponentState(controller, backend), "home", "home", ui.HomeColumn, th),
			ui.NewMessageColumn(ui.NewComponentState(controller, backend), "notifications", "notifications", ui.NotificationsColumn, th),
		},
		backend,
		eventListener,
		config,
	)

	//ui.Renderer.Config.MonospaceFont.Typeface = "Go Mono"
	go func() {
		if err := uinterface.Run(); err != nil {
			log.Fatal(err)
		}
		os.Exit(0)
	}()
	app.Main()
}