package syncodec

import (
	"time"

	"github.com/pion/transport/v3/stdtime"
	"github.com/pion/transport/v3/xtime"
)

var _ Codec = (*PerfectCodec)(nil)

type PerfectCodec struct {
	writer FrameWriter

	targetBitrateBps int
	fps              int

	done        chan struct{}
	timeManager xtime.Manager
}

type PerfectCodecOption func(*PerfectCodec)

func NewPerfectCodec(writer FrameWriter, targetBitrateBps int, options ...PerfectCodecOption) *PerfectCodec {
	c := &PerfectCodec{
		writer:           writer,
		targetBitrateBps: targetBitrateBps,
		fps:              30,
		done:             make(chan struct{}),
		timeManager:      stdtime.Manager{},
	}

	for _, o := range options {
		o(c)
	}
	return c
}

// GetTargetBitrate returns the current target bitrate in bit per second.
func (c *PerfectCodec) GetTargetBitrate() int {
	return c.targetBitrateBps
}

// SetTargetBitrate sets the target bitrate to r bits per second.
func (c *PerfectCodec) SetTargetBitrate(r int) {
	c.targetBitrateBps = r
}

func (c *PerfectCodec) Start() {
	msToNextFrame := time.Duration((1.0/float64(c.fps))*1000.0) * time.Millisecond
	ticker := c.timeManager.NewTicker(msToNextFrame)
	for {
		select {
		case now := <-ticker.C():
			c.writer.WriteFrame(Frame{
				Content:  make([]byte, c.targetBitrateBps/(8.0*c.fps)),
				Duration: msToNextFrame,
			})
			now.Done()
		case <-c.done:
			return
		}
	}
}

func (c *PerfectCodec) Close() error {
	close(c.done)
	return nil
}
