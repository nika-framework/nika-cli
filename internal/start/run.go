package start

import "fmt"

func (a StartApp) Run() error {
	// `-a` with no value: every service at once, each in its own process.
	if a.App == AllApps && a.looksLikeAllApps() {
		plans, err := a.resolveAll()
		if err != nil {
			return err
		}
		if len(plans) == 1 {
			return a.runOne(plans[0])
		}
		return a.runAll(plans)
	}

	resolved, err := a.resolve()
	if err != nil {
		return err
	}
	return a.runOne(resolved)
}

func (a StartApp) runOne(resolved plan) error {
	if resolved.App != "" {
		fmt.Printf("▶️  Starting %s: %s\n", resolved.App, resolved.Build.Cmd)
	}
	if a.WatchMode {
		return a.runWatch(resolved)
	}
	return a.runProduction(resolved)
}

// looksLikeAllApps distinguishes `nika start -a` from `nika start -a api`.
//
// pflag gives the flag its no-value marker in both cases and leaves "api" as a
// positional argument, so the presence of an argument naming a real app is the
// only signal that the user meant one service rather than all of them.
func (a StartApp) looksLikeAllApps() bool {
	return a.Target == ""
}
