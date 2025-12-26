//go:build windows
// +build windows

package monitor

import (
	"os"

	"golang.org/x/term"
	"k8s.io/client-go/tools/remotecommand"
)

type SizeQueue struct {
	ResizeChan   chan remotecommand.TerminalSize
	StopResizing chan struct{}
}

func (s *SizeQueue) Next() *remotecommand.TerminalSize {
	size, ok := <-s.ResizeChan
	if !ok {
		return nil
	}
	return &size
}

func (s *SizeQueue) Stop() {
	close(s.StopResizing)
}

func (s *SizeQueue) MonitorSize() {
	sigCh := make(chan os.Signal, 1)
	// Need to fix this to get it working on windows
	//signal.Notify(sigCh, syscall.SIGWINCH)

	for {
		select {
		case <-sigCh:
			width, height, err := term.GetSize(int(os.Stdout.Fd()))
			if err == nil {
				select {
				case s.ResizeChan <- remotecommand.TerminalSize{Width: uint16(width), Height: uint16(height)}:
				default:
				}
			}
		case <-s.StopResizing:
			close(s.ResizeChan)
			return
		}
	}
}
