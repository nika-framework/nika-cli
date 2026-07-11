package start
 
func (a StartApp) Run() error{
	if a.WatchMode { 
		return a.runWatch()
	} 
	return a.runProduction()
}