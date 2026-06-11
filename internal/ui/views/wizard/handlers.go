package wizard

func (v *View) showError(err error) {
	if v.handlers.Error != nil {
		v.handlers.Error(err)
	}
}
