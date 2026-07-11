package start
 

type StartApp struct { 
	Target string
	WatchMode bool
	PathTom string
}
 
func NewStartApp(Target string,WatchMode bool) *StartApp {
	return &StartApp{ 
		Target: Target,
		WatchMode: WatchMode,
		PathTom: ".nika.toml",
	}
}


