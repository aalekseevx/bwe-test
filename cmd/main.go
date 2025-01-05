package main

import (
	"context"
	"flag"
	"github.com/pion/transport/v3/xtime"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/pion/bwe-test/logging"
	"github.com/pion/bwe-test/receiver"
	"github.com/pion/bwe-test/sender"
)

const initialBitrate = 100_000

func realMain() error {
	mode := flag.String("mode", "sender", "Mode: sender/receiver")
	addr := flag.String("addr", ":4242", "address to listen on /connect to")
	rtpLogFile := flag.String("rtp-log", "", "log RTP to file (use 'stdout' or a file name")
	rtcpLogFile := flag.String("rtcp-log", "", "log RTCP to file (use 'stdout' or a file name")
	ccLogFile := flag.String("cc-log", "", "log congestion control target bitrate")
	flag.Parse()

	tm := xtime.StdTimeManager{}

	if *mode == "receiver" {
		return receive(*addr, *rtpLogFile, *rtcpLogFile, tm)
	}
	if *mode == "sender" {
		return send(*addr, *rtpLogFile, *rtcpLogFile, *ccLogFile, tm)
	}

	log.Fatalf("invalid mode: %s\n", *mode)
	return nil
}

func receive(addr, rtpLogFile, rtcpLogFile string, tm xtime.TimeManager) error {
	options := []receiver.Option{
		receiver.PacketLogWriter(os.Stdout, os.Stdout, tm),
		receiver.DefaultInterceptors(),
		receiver.SetTimeManager(tm),
	}
	var rtpLogger io.WriteCloser
	var rtcpLogger io.WriteCloser
	var err error
	if rtpLogFile != "" {
		rtpLogger, err = logging.GetLogFile(rtpLogFile)
		if err != nil {
			return err
		}
		defer rtpLogger.Close()
	}
	if rtcpLogFile != "" {
		rtcpLogger, err = logging.GetLogFile(rtcpLogFile)
		if err != nil {
			return err
		}
		defer rtcpLogger.Close()
	}
	if rtpLogger != nil || rtcpLogger != nil {
		options = append(options, receiver.PacketLogWriter(rtpLogger, rtcpLogger, tm))
	}
	r, err := receiver.NewReceiver(options...)
	if err != nil {
		return err
	}
	err = r.SetupPeerConnection()
	if err != nil {
		return err
	}
	http.Handle("/sdp", r.SDPHandler())
	log.Fatal(http.ListenAndServe(addr, nil))
	return nil
}

func send(addr, rtpLogFile, rtcpLogFile, ccLogFile string, tm xtime.TimeManager) error {
	options := []sender.Option{
		sender.DefaultInterceptors(),
		sender.GCC(initialBitrate),
		sender.SetTimeManager(tm),
	}
	var rtpLogger io.WriteCloser
	var rtcpLogger io.WriteCloser
	var err error
	if rtpLogFile != "" {
		rtpLogger, err = logging.GetLogFile(rtpLogFile)
		if err != nil {
			return err
		}
		defer rtpLogger.Close()
	}
	if rtcpLogFile != "" {
		rtcpLogger, err = logging.GetLogFile(rtcpLogFile)
		if err != nil {
			return err
		}
		defer rtcpLogger.Close()
	}
	if ccLogFile != "" {
		var ccLogger io.WriteCloser
		ccLogger, err = logging.GetLogFile(ccLogFile)
		if err != nil {
			return err
		}
		defer ccLogger.Close()
		options = append(options, sender.CCLogWriter(ccLogger))
	}
	if rtpLogger != nil || rtcpLogger != nil {
		options = append(options, sender.PacketLogWriter(rtpLogger, rtcpLogger, tm))
	}
	s, err := sender.NewSender(
		sender.NewStatisticalEncoderSource(),
		options...,
	)
	if err != nil {
		return err
	}
	err = s.SetupPeerConnection()
	if err != nil {
		return err
	}
	err = s.SignalHTTP(addr, "sdp")
	if err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	defer func() {
		signal.Stop(sigs)
		cancel()
	}()
	go func() {
		select {
		case <-sigs:
			cancel()
			log.Println("cancel called")
		case <-ctx.Done():
		}
	}()

	return s.Start(ctx)
}

func main() {
	err := realMain()
	if err != nil {
		log.Fatal(err)
	}
}
