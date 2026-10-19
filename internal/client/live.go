package client

import (
	"fmt"
	"sync"
	"time"
)

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// liveLine is a single status line redrawn in place with a spinner and the
// elapsed time. It only draws in the interactive shell on a real terminal;
// elsewhere it is inert and callers print plain lines instead.
type liveLine struct {
	mu       sync.Mutex
	text     string
	start    time.Time
	active   bool
	stop     chan struct{}
	stopOnce sync.Once
	wg       sync.WaitGroup
}

func newLiveLine() *liveLine {
	l := &liveLine{start: time.Now()}
	if !Interactive || !stdoutIsTerminal() {
		return l
	}
	l.active = true
	l.stop = make(chan struct{})
	l.wg.Add(1)
	go l.loop()
	return l
}

func (l *liveLine) loop() {
	defer l.wg.Done()
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for frame := 0; ; frame++ {
		select {
		case <-l.stop:
			return
		case <-ticker.C:
			l.mu.Lock()
			elapsed := time.Since(l.start).Truncate(time.Second)
			fmt.Printf("\r\033[K%s %s %s", Cyan(spinnerFrames[frame%len(spinnerFrames)]), l.text, Dim(elapsed.String()))
			l.mu.Unlock()
		}
	}
}

func (l *liveLine) Set(text string) {
	l.mu.Lock()
	l.text = text
	l.mu.Unlock()
}

func (l *liveLine) Stop() {
	if !l.active {
		return
	}
	l.stopOnce.Do(func() {
		close(l.stop)
		l.wg.Wait()
		fmt.Print("\r\033[K")
	})
}
