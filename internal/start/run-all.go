package start

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// Running every service at once.
//
// A microservice workspace is normally three or four processes that only make
// sense together — an API, a gRPC service, a Kafka consumer. Starting them from
// four terminals is the state of the art this replaces: `nika start -a` runs
// them all, tags each line of output with the service it came from, and stops
// them together on Ctrl+C.

// service is one running app in a multi-app run.
type service struct {
	name  string
	dir   string // app root relative to project root, "" for a root-level app
	build BuildConfig
	root  string
	color string

	mu   sync.Mutex
	cmd  *exec.Cmd
	done chan struct{}
}

// prefixColors tags each service's output. They repeat past six services,
// which is fine: the name is on every line, the colour only helps the eye.
var prefixColors = []string{
	"\033[36m", // cyan
	"\033[35m", // magenta
	"\033[32m", // green
	"\033[33m", // yellow
	"\033[34m", // blue
	"\033[31m", // red
}

const colorReset = "\033[0m"

// runAll starts every plan and blocks until interrupted.
func (a StartApp) runAll(plans []plan) error {
	root := plans[0].Config.Root
	if strings.TrimSpace(root) == "" {
		root = "."
	}

	services := make([]*service, 0, len(plans))
	width := 0
	for _, p := range plans {
		if len(p.App) > width {
			width = len(p.App)
		}
	}
	for i, p := range plans {
		services = append(services, &service{
			name:  p.App,
			dir:   p.Dir,
			build: p.Build,
			root:  root,
			color: prefixColors[i%len(prefixColors)],
		})
	}

	fmt.Printf("▶️  Starting %d services:\n", len(services))
	for _, svc := range services {
		fmt.Printf("   %s%-*s%s  %s\n", svc.color, width, svc.name, colorReset, svc.build.Cmd)
	}

	for _, svc := range services {
		svc.start(width)
	}

	if a.WatchMode {
		fmt.Println("🔄 Watch mode enabled – watching for changes...")
		return watchAll(root, plans[0].Build, services, width)
	}

	// Without --watch there is nothing to react to, so just wait for a signal
	// or for every service to exit on its own.
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt)

	exited := make(chan struct{})
	go func() {
		for _, svc := range services {
			svc.wait()
		}
		close(exited)
	}()

	select {
	case <-signals:
		fmt.Println("\n🛑 Shutting down...")
	case <-exited:
	}
	stopAll(services)
	return nil
}

// watchAll restarts services when their files change.
//
// A change under apps/<name>/ restarts only that service; a change anywhere
// else — shared code at the root, internal/, go.mod — restarts everything,
// because there is no cheap way to know which services import it.
func watchAll(root string, build BuildConfig, services []*service, width int) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("failed to create watcher: %w", err)
	}
	defer watcher.Close()

	excludeRegexes := make([]*regexp.Regexp, 0, len(build.ExcludeRegex))
	for _, pattern := range build.ExcludeRegex {
		re, err := regexp.Compile(pattern)
		if err != nil {
			return fmt.Errorf("invalid exclude_regex %q: %w", pattern, err)
		}
		excludeRegexes = append(excludeRegexes, re)
	}
	if err := addDirRecursively(watcher, root, build.ExcludeDir); err != nil {
		return fmt.Errorf("failed to watch directory: %w", err)
	}

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt)

	delay := time.Duration(build.Delay) * time.Millisecond
	if delay <= 0 {
		delay = time.Second
	}

	var mu sync.Mutex
	pending := map[*service]bool{}
	var timer *time.Timer

	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				return nil
			}
			if event.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Remove|fsnotify.Rename) == 0 {
				continue
			}
			if !shouldWatch(event.Name, build, excludeRegexes) {
				continue
			}
			affected := servicesFor(root, event.Name, services)
			if len(affected) == 0 {
				continue
			}

			mu.Lock()
			for _, svc := range affected {
				pending[svc] = true
			}
			changed := event.Name
			if timer != nil {
				timer.Stop()
			}
			timer = time.AfterFunc(delay, func() {
				mu.Lock()
				restart := make([]*service, 0, len(pending))
				for svc := range pending {
					restart = append(restart, svc)
				}
				pending = map[*service]bool{}
				mu.Unlock()

				names := make([]string, len(restart))
				for i, svc := range restart {
					names[i] = svc.name
				}
				fmt.Printf("📁 Change detected: %s → restarting %s\n", changed, strings.Join(names, ", "))
				for _, svc := range restart {
					svc.start(width)
				}
			})
			mu.Unlock()

		case err, ok := <-watcher.Errors:
			if !ok {
				return nil
			}
			fmt.Printf("⚠️ Watcher error: %v\n", err)

		case <-signals:
			fmt.Println("\n🛑 Shutting down...")
			mu.Lock()
			if timer != nil {
				timer.Stop()
			}
			mu.Unlock()
			stopAll(services)
			return nil
		}
	}
}

// servicesFor maps a changed file to the services that must restart.
func servicesFor(root, changed string, services []*service) []*service {
	relative, err := filepath.Rel(root, changed)
	if err != nil {
		return services
	}
	relative = filepath.ToSlash(relative)

	for _, svc := range services {
		if svc.dir != "" && strings.HasPrefix(relative, svc.dir+"/") {
			return []*service{svc}
		}
	}
	return services
}

func stopAll(services []*service) {
	var wg sync.WaitGroup
	for _, svc := range services {
		wg.Add(1)
		go func(s *service) {
			defer wg.Done()
			s.stop()
		}(svc)
	}
	wg.Wait()
}

// start stops any previous process for this service and launches a new one.
func (s *service) start(width int) {
	s.stop()

	s.runHooks(s.build.PreCmd, width)

	command := strings.TrimSpace(s.build.Cmd)
	if command == "" {
		command = "go run ."
	}
	parts := strings.Fields(command)
	args := append(append([]string{}, parts[1:]...), s.build.Args...)

	cmd := exec.Command(parts[0], args...)
	cmd.Dir = s.root
	cmd.Env = buildEnv(s.build)
	cmd.Stdout = s.writer(os.Stdout, width)
	cmd.Stderr = s.writer(os.Stderr, width)
	configureProcess(cmd)

	if err := cmd.Start(); err != nil {
		fmt.Printf("%s❌ %s: failed to start: %v%s\n", s.color, s.name, err, colorReset)
		return
	}

	done := make(chan struct{})
	s.mu.Lock()
	s.cmd = cmd
	s.done = done
	s.mu.Unlock()

	go func() {
		if err := cmd.Wait(); err != nil {
			if _, ok := err.(*exec.ExitError); !ok {
				fmt.Printf("%s⚠️  %s exited with error: %v%s\n", s.color, s.name, err, colorReset)
			}
		}
		s.runHooks(s.build.PostCmd, width)
		close(done)
	}()
}

// stop terminates the running process and waits for it to be reaped, so a
// restart cannot leave two copies of a service bound to the same port.
func (s *service) stop() {
	s.mu.Lock()
	cmd, done := s.cmd, s.done
	s.cmd, s.done = nil, nil
	s.mu.Unlock()

	if cmd == nil || cmd.Process == nil {
		return
	}
	stopProcess(cmd)
	if done != nil {
		<-done
	}
}

// wait blocks until the current process exits.
func (s *service) wait() {
	s.mu.Lock()
	done := s.done
	s.mu.Unlock()
	if done != nil {
		<-done
	}
}

func (s *service) runHooks(commands []string, width int) {
	for _, entry := range commands {
		if strings.TrimSpace(entry) == "" {
			continue
		}
		parts := strings.Fields(entry)
		cmd := exec.Command(parts[0], parts[1:]...)
		cmd.Dir = s.root
		cmd.Env = buildEnv(s.build)
		cmd.Stdout = s.writer(os.Stdout, width)
		cmd.Stderr = s.writer(os.Stderr, width)
		if err := cmd.Run(); err != nil {
			fmt.Printf("%s⚠️  %s: command failed: %s (%v)%s\n", s.color, s.name, entry, err, colorReset)
		}
	}
}

func (s *service) writer(out io.Writer, width int) io.Writer {
	return &prefixWriter{
		out:    out,
		prefix: fmt.Sprintf("%s%-*s%s │ ", s.color, width, s.name, colorReset),
	}
}

// prefixWriter tags every line with the service that produced it.
//
// It buffers a partial line rather than prefixing each Write, because a
// process that prints a progress line in several writes would otherwise get a
// service tag in the middle of it.
type prefixWriter struct {
	out    io.Writer
	prefix string

	mu      sync.Mutex
	partial []byte
}

func (w *prefixWriter) Write(data []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.partial = append(w.partial, data...)
	for {
		index := bytes.IndexByte(w.partial, '\n')
		if index < 0 {
			break
		}
		line := w.partial[:index]
		w.partial = w.partial[index+1:]
		if _, err := fmt.Fprintf(w.out, "%s%s\n", w.prefix, strings.TrimRight(string(line), "\r")); err != nil {
			return len(data), err
		}
	}
	// A process that never terminates its last line would otherwise hold it
	// forever; flush once the buffer stops looking like a partial line.
	if len(w.partial) > 4096 {
		if _, err := fmt.Fprintf(w.out, "%s%s\n", w.prefix, string(w.partial)); err != nil {
			return len(data), err
		}
		w.partial = nil
	}
	return len(data), nil
}
