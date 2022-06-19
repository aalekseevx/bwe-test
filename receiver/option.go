package receiver

import (
	"io"
	"time"

	"github.com/pion/bwe-test/logging"
	"github.com/pion/interceptor/pkg/packetdump"
	"github.com/pion/transport/vnet"
	"github.com/pion/webrtc/v3"
)

type Option func(*Receiver) error

func PacketLogWriter(rtpWriter, rtcpWriter io.Writer) Option {
	return func(r *Receiver) error {
		formatter := logging.RTPFormatter{}
		rtpLogger, err := packetdump.NewReceiverInterceptor(
			packetdump.RTPFormatter(formatter.RTPFormat),
			packetdump.RTPWriter(rtpWriter),
		)
		if err != nil {
			return err
		}
		rtcpLogger, err := packetdump.NewSenderInterceptor(
			packetdump.RTCPFormatter(logging.RTCPFormat),
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
		r.settingEngine.SetVNet(v)
		r.settingEngine.SetICETimeouts(time.Second, time.Second, 200*time.Millisecond)
		r.settingEngine.SetNAT1To1IPs(publicIPs, webrtc.ICECandidateTypeHost)
		return nil
	}
}
