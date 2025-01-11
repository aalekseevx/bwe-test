package receiver

import (
	"io"
	"time"

	"github.com/pion/transport/v3/xtime"

	"github.com/pion/interceptor/pkg/packetdump"
	"github.com/pion/webrtc/v4"

	"github.com/pion/bwe-test/logging"
	"github.com/pion/transport/v3/vnet"
)

type Option func(*Receiver) error

func PacketLogWriter(rtpWriter, rtcpWriter io.Writer, tm xtime.Manager) Option {
	return func(r *Receiver) error {
		formatter := logging.RTPFormatter{
			TimeManager: tm,
		}
		rtpLogger, err := packetdump.NewReceiverInterceptor(
			packetdump.RTPFormatter(formatter.RTPFormat),
			packetdump.RTPWriter(rtpWriter),
		)
		if err != nil {
			return err
		}
		rtcpFormatter := logging.RTCPFormatter{
			TimeManager: tm,
		}
		rtcpLogger, err := packetdump.NewSenderInterceptor(
			packetdump.RTCPFormatter(rtcpFormatter.RTCPFormat),
			packetdump.RTCPWriter(rtcpWriter),
		)
		if err != nil {
			return err
		}
		r.registry.Add(rtpLogger)
		r.registry.Add(rtcpLogger)
		return nil
	}
}

func DefaultInterceptors() Option {
	return func(r *Receiver) error {
		return webrtc.RegisterDefaultInterceptors(r.mediaEngine, r.registry)
	}
}

func SetVnet(v *vnet.Net, publicIPs []string) Option {
	return func(r *Receiver) error {
		r.settingEngine.SetNet(v)
		r.settingEngine.SetICETimeouts(time.Second, time.Second, 200*time.Millisecond)
		r.settingEngine.SetNAT1To1IPs(publicIPs, webrtc.ICECandidateTypeHost)
		return nil
	}
}

func SetTimeManager(tm xtime.Manager) Option {
	return func(r *Receiver) error {
		r.timeManager = tm
		return nil
	}
}
