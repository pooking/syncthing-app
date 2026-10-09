//go:build !windows && !darwin && (android || ios)

package autostart

func enable(string, []string) error { return nil }
func disable() error                { return nil }
func isEnabled() (bool, error)      { return false, nil }
