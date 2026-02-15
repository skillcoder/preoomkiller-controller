package app

import (
	"context"
	"log/slog"
	"testing"
	"time"
)

type allChannelsCloseCase struct {
	name                         string
	giveNumChannels              int
	giveContextCancelBeforeClose bool
	wantClosed                   bool
}

func TestAllChannelsClose(t *testing.T) {
	logger := slog.Default()

	tests := []allChannelsCloseCase{
		{
			name:            "zero channels closes immediately",
			giveNumChannels: 0,
			wantClosed:      true,
		},
		{
			name:            "one channel closes when it closes",
			giveNumChannels: 1,
			wantClosed:      true,
		},
		{
			name:            "two channels close when both close",
			giveNumChannels: 2,
			wantClosed:      true,
		},
		{
			name:                         "context cancelled then channels close",
			giveNumChannels:              2,
			giveContextCancelBeforeClose: true,
			wantClosed:                   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := t.Context()

			if tt.giveContextCancelBeforeClose {
				var cancel context.CancelFunc

				ctx, cancel = context.WithCancel(ctx)
				cancel()
			}

			chans := make([]<-chan struct{}, 0, tt.giveNumChannels)
			readyChans := make([]chan struct{}, 0, tt.giveNumChannels)

			for range tt.giveNumChannels {
				ch := make(chan struct{})

				readyChans = append(readyChans, ch)
				chans = append(chans, ch)
			}

			out := allChannelsClose(ctx, logger, chans...)

			if tt.giveNumChannels == 0 {
				select {
				case <-out:
				case <-time.After(100 * time.Millisecond):
					t.Fatal("expected out channel to close immediately")
				}

				return
			}

			for _, ch := range readyChans {
				close(ch)
			}

			select {
			case <-out:
			case <-time.After(500 * time.Millisecond):
				t.Fatal("expected out channel to close after all input channels closed")
			}
		})
	}
}
