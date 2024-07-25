//go:build linux || darwin
// +build linux darwin

package main

import (
	"os"
	"os/signal"
	"syscall"

	"golang.org/x/term"
	"k8s.io/client-go/tools/remotecommand"
)

type sizeQueue struct {
	resizeChan   chan remotecommand.TerminalSize
	stopResizing chan struct{}
}

func (s *sizeQueue) Next() *remotecommand.TerminalSize {
	size, ok := <-s.resizeChan
	if !ok {
		return nil
	}
	return &size
}

func (s *sizeQueue) Stop() {
	close(s.stopResizing)
}

func (s *sizeQueue) MonitorSize() {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGWINCH)

	for {
		select {
		case <-sigCh:
			width, height, err := term.GetSize(int(os.Stdout.Fd()))
			if err == nil {
				select {
				case s.resizeChan <- remotecommand.TerminalSize{Width: uint16(width), Height: uint16(height)}:
				default:
				}
			}
		case <-s.stopResizing:
			close(s.resizeChan)
			return
		}
	}
}
