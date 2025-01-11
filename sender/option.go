package sender

import (
	"io"
	"time"

	"github.com/pion/transport/v3/xtime"

	"github.com/pion/webrtc/v4"

	"github.com/pion/bwe-test/logging"
	"github.com/pion/interceptor/pkg/cc"
	"github.com/pion/interceptor/pkg/gcc"
	"github.com/pion/interceptor/pkg/packetdump"
	"github.com/pion/transport/v3/vnet"
)

type Option func(*Sender) error

func PacketLogWriter(rtpWriter, rtcpWriter io.Writer, tm xtime.Manager) Option {
	return func(s *Sender) error {
		formatter := logging.RTPFormatter{
			TimeManager: tm,
		}
		rtpLogger, err := packetdump.NewSenderInterceptor(
			packetdump.RTPFormatter(formatter.RTPFormat),
			packetdump.RTPWriter(rtpWriter),
		)
		if err != nil {
			return err
		}
		rtcpFormatter := logging.RTCPFormatter{
			TimeManager: tm,
		}
		rtcpLogger, err := packetdump.NewReceiverInterceptor(
			packetdump.RTCPFormatter(rtcpFormatter.RTCPFormat),
			packetdump.RTCPWriter(rtcpWriter),
		)
		if err != nil {
			return err
		}
		s.registry.Add(rtpLogger)
		s.registry.Add(rtcpLogger)
		return nil
	}
}

func DefaultInterceptors() Option {
	return func(s *Sender) error {
		return webrtc.RegisterDefaultInterceptors(s.mediaEngine, s.registry)
	}
}

func CCLogWriter(w io.Writer) Option {
	return func(s *Sender) error {
		s.ccLogWriter = w
		return nil
	}
}

func GCC(initialBitrate int, tm xtime.Manager) Option {
	return func(s *Sender) error {
		controller, err := cc.NewInterceptor(func() (cc.BandwidthEstimator, error) {
			return gcc.NewSendSideBWE(
				gcc.SendSideBWEInitialBitrate(initialBitrate),
				gcc.SendSideTimeManager(tm),
			)
		})
		if err != nil {
			return err
		}
		controller.OnNewPeerConnection(func(_ string, estimator cc.BandwidthEstimator) {
			go func() {
				s.estimatorChan <- estimator
			}()
		})
		s.registry.Add(controller)
		if err = webrtc.ConfigureTWCCHeaderExtensionSender(s.mediaEngine, s.registry); err != nil {
			return err
		}
		return nil
	}
}

func SetVnet(v *vnet.Net, publicIPs []string) Option {
	return func(s *Sender) error {
		s.settingEngine.SetNet(v)
		s.settingEngine.SetICETimeouts(time.Second, time.Second, 200*time.Millisecond)
		s.settingEngine.SetNAT1To1IPs(publicIPs, webrtc.ICECandidateTypeHost)
		return nil
	}
}

func SetMediaSource(source MediaSource) Option {
	return func(s *Sender) error {
		s.source = source
		return nil
	}
}

func SetTimeManager(manager xtime.Manager) Option {
	return func(s *Sender) error {
		s.timeManager = manager
		return nil
	}
}
