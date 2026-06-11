package classifier

import "fyne.io/fyne/v2"

// answerKeys maps the number-row keys to a 0-based answer index.
var answerKeys = map[fyne.KeyName]int{
	fyne.Key1: 0,
	fyne.Key2: 1,
	fyne.Key3: 2,
	fyne.Key4: 3,
	fyne.Key5: 4,
	fyne.Key6: 5,
	fyne.Key7: 6,
	fyne.Key8: 7,
	fyne.Key9: 8,
}

// HandleKey routes raw key events: Left/Right navigate records and
// the number row selects answers.
func (v *View) HandleKey(key *fyne.KeyEvent) {
	switch key.Name {
	case fyne.KeyLeft:
		v.Previous()
		return
	case fyne.KeyRight:
		v.Skip()
		return
	}

	if index, ok := answerKeys[key.Name]; ok {
		v.SelectAnswerByIndex(index)
	}
}

func (v *View) showError(err error) {
	if v.handlers.Error != nil {
		v.handlers.Error(err)
	}
}

func (v *View) showInformation(title, message string) {
	if v.handlers.Information != nil {
		v.handlers.Information(title, message)
	}
}
