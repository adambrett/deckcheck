package welcome

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/lang"

	"github.com/adambrett/go-fyne/pkg/browse"
	"github.com/adambrett/go-fyne/pkg/launcher"
	launcherTheme "github.com/adambrett/go-fyne/pkg/launcher/theme"
	"github.com/adambrett/go-fyne/pkg/recent"

	deckTheme "github.com/adambrett/deckcheck/internal/fyneui/theme"
	"github.com/adambrett/deckcheck/internal/ui/views"
)

// Compile-time check: welcome.View satisfies the views.View contract.
var _ views.View = (*View)(nil)

const welcomeTitle = "DeckCheck"

// Handlers bundles the welcome screen's outward callbacks. Create
// starts the new-project wizard; Open opens the project at path.
// Either field may be nil; the view guards before invoking.
type Handlers struct {
	Create func()
	Open   func(path string)
}

// View wraps the launcher-backed welcome screen.
type View struct {
	launcher *launcher.Launcher
}

// Config holds the welcome view's dependencies. Recents feeds the
// recent-projects list, Icon is the logo, and Picker backs the Open
// Project flow.
type Config struct {
	Recents  *recent.Recent
	Icon     fyne.Resource
	Picker   browse.Picker
	Handlers Handlers
}

// New builds the welcome view from its dependencies.
func New(cfg Config) *View {
	handlers := cfg.Handlers

	return &View{
		launcher: launcher.New(
			cfg.Recents,
			nil, // no item-preview callback; DeckCheck cards have no thumbnails
			func() (recent.Item, bool) {
				if handlers.Create != nil {
					handlers.Create()
				}
				// The launcher offers to record the created item as a
				// recent; project creation flows through the wizard, so
				// nothing is recorded here.
				return recent.Item{}, false
			},
			func(item recent.Item) {
				if handlers.Open != nil {
					handlers.Open(item.Path)
				}
			},
			launcher.WithTitle(welcomeTitle),
			launcher.WithCreateLabel(lang.L("New Project")),
			launcher.WithOpenLabel(lang.L("Open Project")),
			launcher.WithRecentTitle(lang.L("Recent Projects")),
			launcher.WithEmptyRecentText(lang.L("No recent projects yet")),
			launcher.WithLogo(cfg.Icon),
			launcher.WithFilePicker(cfg.Picker),
			launcher.WithSplitOffset(welcomeSplitOffset),
			launcher.WithTheme(launcherTheme.Theme{
				Background:          deckTheme.Gray950,
				CardBackground:      deckTheme.Gray800,
				CardHoverBackground: deckTheme.Gray700,
				CardBorder:          deckTheme.Gray700,
				CardHoverBorder:     deckTheme.Yellow500,
				PrimaryText:         deckTheme.Gray200,
				SecondaryText:       deckTheme.Gray400,
				MutedText:           deckTheme.Gray400,
			}),
		),
	}
}

// welcomeSplitOffset balances the action column against the recents
// list: enough room for the two action cards without starving the
// recent-projects pane.
const welcomeSplitOffset = 0.4
