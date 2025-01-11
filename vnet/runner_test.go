package vnet

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/pion/transport/v3/stdtime"
	"github.com/pion/transport/v3/vtime"
	"github.com/pion/transport/v3/xtime"

	"github.com/pion/bwe-test/logging"
	"github.com/pion/bwe-test/sender"

	"github.com/stretchr/testify/assert"

	"github.com/pion/bwe-test/receiver"
	"github.com/pion/transport/v3/vnet"
)

type senderMode int

const (
	simulcastSenderMode senderMode = iota
	abrSenderMode
)

func TestVnetRunnerSimulcast(t *testing.T) {
	VnetRunner(t, simulcastSenderMode)
}

func TestVnetRunnerABR(t *testing.T) {
	VnetRunner(t, abrSenderMode)
}

const useVTime = true

func VnetRunner(t *testing.T, mode senderMode) {
	t.Run("VariableAvailableCapacitySingleFlow", func(t *testing.T) {
		var tm xtime.Manager
		if useVTime {
			tm = vtime.NewSimulator(time.Unix(0, 0))
		} else {
			tm = stdtime.Manager{}
		}

		nm, err := NewManager(tm)
		assert.NoError(t, err)

		err = os.MkdirAll(fmt.Sprintf("data/%v", t.Name()), os.ModePerm)
		assert.NoError(t, err)

		s, r, teardown := setupSimpleFlow(t, nm, tm, mode, 0)
		defer teardown()

		//time.Sleep(time.Second)

		if vtm, ok := tm.(*vtime.Simulator); ok {
			vtm.Start()
			defer vtm.Stop()
		}

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go func() {
			err = s.Start(ctx)
			assert.NoError(t, err)
		}()
		defer func() {
			err = r.Close()
			assert.NoError(t, err)
		}()

		c := pathCharacteristics{
			referenceCapacity: 1 * vnet.MBit,
			phases: []phase{
				{
					duration:      40 * time.Second,
					capacityRatio: 1.0,
					maxBurst:      160 * vnet.KBit,
				},
				{
					duration:      20 * time.Second,
					capacityRatio: 2.5,
					maxBurst:      160 * vnet.KBit,
				},
				{
					duration:      20 * time.Second,
					capacityRatio: 0.6,
					maxBurst:      160 * vnet.KBit,
				},
				{
					duration:      20 * time.Second,
					capacityRatio: 1.0,
					maxBurst:      160 * vnet.KBit,
				},
			},
		}
		runNetworkSimulation(t, c, nm, tm)
	})

	t.Run("VariableAvailableCapacityMultipleFlows", func(t *testing.T) {
		var tm xtime.Manager
		if useVTime {
			tm = vtime.NewSimulator(time.Unix(0, 0))
		} else {
			tm = stdtime.Manager{}
		}

		nm, err := NewManager(tm)
		assert.NoError(t, err)

		err = os.MkdirAll(fmt.Sprintf("data/%v", t.Name()), os.ModePerm)
		assert.NoError(t, err)

		for i := 0; i < 2; i++ {
			s, r, teardown := setupSimpleFlow(t, nm, tm, mode, i)
			defer teardown()

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			go func() {
				err = s.Start(ctx)
				assert.NoError(t, err)
			}()

			defer func() {
				err = r.Close()
				assert.NoError(t, err)
			}()
		}

		if vtm, ok := tm.(*vtime.Simulator); ok {
			vtm.Start()
			defer vtm.Stop()
		}

		c := pathCharacteristics{
			referenceCapacity: 1 * vnet.MBit,
			phases: []phase{
				{
					duration:      25 * time.Second,
					capacityRatio: 2.0,
					maxBurst:      160 * vnet.KBit,
				},

				{
					duration:      25 * time.Second,
					capacityRatio: 1.0,
					maxBurst:      160 * vnet.KBit,
				},
				{
					duration:      25 * time.Second,
					capacityRatio: 1.75,
					maxBurst:      160 * vnet.KBit,
				},
				{
					duration:      25 * time.Second,
					capacityRatio: 0.5,
					maxBurst:      160 * vnet.KBit,
				},
				{
					duration:      25 * time.Second,
					capacityRatio: 1.0,
					maxBurst:      160 * vnet.KBit,
				},
			},
		}
		runNetworkSimulation(t, c, nm, tm)
	})
}

type pathCharacteristics struct {
	referenceCapacity int
	phases            []phase
}

type phase struct {
	duration      time.Duration
	capacityRatio float64
	maxBurst      int
}

func runNetworkSimulation(t *testing.T, c pathCharacteristics, nm *NetworkManager, tm xtime.Manager) {
	done := func() {}
	for _, phase := range c.phases {
		t.Logf("enter next phase: %v\n", phase)
		nm.SetCapacity(
			int(float64(c.referenceCapacity)*phase.capacityRatio),
			phase.maxBurst,
		)
		done()
		phaseChange := <-tm.NewTimer(phase.duration, true).C()
		done = func() {
			phaseChange.Done()
		}
	}
}

func setupSimpleFlow(t *testing.T, nm *NetworkManager, tm xtime.Manager, mode senderMode, id int) (*sender.Sender, *receiver.Receiver, func()) {
	leftVnet, publicIPLeft, err := nm.GetLeftNet()
	assert.NoError(t, err)
	rightVnet, publicIPRight, err := nm.GetRightNet()
	assert.NoError(t, err)

	senderRTPLogger, err := logging.GetLogFile(fmt.Sprintf("data/%v/%v_sender_rtp.log", t.Name(), id))
	assert.NoError(t, err)
	senderRTCPLogger, err := logging.GetLogFile(fmt.Sprintf("data/%v/%v_sender_rtcp.log", t.Name(), id))
	assert.NoError(t, err)
	ccLogger, err := logging.GetLogFile(fmt.Sprintf("data/%v/%v_cc.log", t.Name(), id))
	assert.NoError(t, err)

	var s *sender.Sender
	switch mode {
	case abrSenderMode:
		s, err = sender.NewSender(
			sender.NewStatisticalEncoderSource(tm),
			sender.SetVnet(leftVnet, []string{publicIPLeft}),
			sender.PacketLogWriter(senderRTPLogger, senderRTCPLogger, tm),
			sender.GCC(100_000, tm),
			sender.CCLogWriter(ccLogger),
			sender.SetTimeManager(tm),
		)
		assert.NoError(t, err)
	case simulcastSenderMode:
		s, err = sender.NewSender(
			sender.NewSimulcastFilesSource(tm),
			sender.SetVnet(leftVnet, []string{publicIPLeft}),
			sender.PacketLogWriter(senderRTPLogger, senderRTCPLogger, tm),
			sender.GCC(100_000, tm),
			sender.CCLogWriter(ccLogger),
			sender.SetTimeManager(tm),
		)
		assert.NoError(t, err)
	default:
		assert.Fail(t, "invalid sender mode", mode)
	}

	receiverRTPLogger, err := logging.GetLogFile(fmt.Sprintf("data/%v/%v_receiver_rtp.log", t.Name(), id))
	assert.NoError(t, err)
	receiverRTCPLogger, err := logging.GetLogFile(fmt.Sprintf("data/%v/%v_receiver_rtcp.log", t.Name(), id))
	assert.NoError(t, err)

	r, err := receiver.NewReceiver(
		receiver.SetVnet(rightVnet, []string{publicIPRight}),
		receiver.PacketLogWriter(receiverRTPLogger, receiverRTCPLogger, tm),
		receiver.DefaultInterceptors(),
		receiver.SetTimeManager(tm),
	)
	assert.NoError(t, err)

	err = s.SetupPeerConnection()
	assert.NoError(t, err)

	offer, err := s.CreateOffer()
	assert.NoError(t, err)

	err = r.SetupPeerConnection()
	assert.NoError(t, err)

	answer, err := r.AcceptOffer(offer)
	assert.NoError(t, err)

	err = s.AcceptAnswer(answer)
	assert.NoError(t, err)

	s.WaitConnected()
	r.WaitConnected()

	return s, r, func() {
		assert.NoError(t, senderRTPLogger.Close())
		assert.NoError(t, senderRTCPLogger.Close())
		assert.NoError(t, ccLogger.Close())
		assert.NoError(t, receiverRTPLogger.Close())
		assert.NoError(t, receiverRTCPLogger.Close())
	}
}
