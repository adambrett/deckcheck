package main

import (
	"log"

	"github.com/adambrett/deckcheck/internal/ui/app"
	"github.com/adambrett/deckcheck/internal/ui/dependencies"
)

func main() {
	deps, err := dependencies.New()
	if err != nil {
		log.Fatalf("failed to create dependencies: %v", err)
	}

	app.New(app.Config{
		App:           deps.App,
		Window:        deps.Window,
		Content:       deps.Content,
		Icon:          deps.Icon,
		Picker:        deps.Picker,
		ProjectPicker: deps.ProjectPicker,
		Recents:       deps.Recents,
	}).Run()
}
