// SPDX-FileCopyrightText: 2025 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/pion/logging"
	"github.com/pion/transport/v3/vnet"
)

type senderMode int

const (
	simulcastSenderMode senderMode = iota
	abrSenderMode
)

type flowMode int

const (
	singleFlowMode flowMode = iota
	multipleFlowsMode
)

func main() {
	testCases := []struct {
		name       string
		senderMode senderMode
		flowMode   flowMode
	}{
		{
			name:       "TestVnetRunnerABR/VariableAvailableCapacitySingleFlow",
			senderMode: abrSenderMode,
			flowMode:   singleFlowMode,
		},
		{
			name:       "TestVnetRunnerABR/VariableAvailableCapacityMultipleFlows",
			senderMode: abrSenderMode,
			flowMode:   multipleFlowsMode,
		},
		{
			name:       "TestVnetRunnerSimulcast/VariableAvailableCapacitySingleFlow",
			senderMode: simulcastSenderMode,
			flowMode:   singleFlowMode,
		},
		{
			name:       "TestVnetRunnerSimulcast/VariableAvailableCapacityMultipleFlows",
			senderMode: simulcastSenderMode,
			flowMode:   multipleFlowsMode,
		},
	}

	logger := logging.NewDefaultLoggerFactory().NewLogger("bwe_test_runner")
	for _, t := range testCases {
		runner := Runner{
			logger:     logger,
			name:       t.name,
			senderMode: t.senderMode,
			flowMode:   t.flowMode,
		}
		err := runner.Run()
		if err != nil {
			logger.Errorf("runner error: %v", err)
		}
	}
}

type Runner struct {
	logger     logging.LeveledLogger
	name       string
	senderMode senderMode
	flowMode   flowMode
}

func (r *Runner) Run() error {
	switch r.flowMode {
	case singleFlowMode:
		err := r.runVariableAvailableCapacitySingleFlow()
		if err != nil {
			return fmt.Errorf("run variable availiable capacity single flow: %w", err)
		}
	case multipleFlowsMode:
		err := r.runVariableAvailableCapacityMultipleFlows()
		if err != nil {
			return fmt.Errorf("run variable availiable capacity multiple flows: %w", err)
		}
	default:
		return fmt.Errorf("unknown flow mode: %v", r.flowMode)
	}
	return nil
}

func (r *Runner) runVariableAvailableCapacitySingleFlow() error {
	nm, err := NewManager()
	if err != nil {
		return fmt.Errorf("new manager: %w", err)
	}

	dataDir := fmt.Sprintf("data/%v", r.name)
	err = os.MkdirAll(dataDir, os.ModePerm)
	if err != nil {
		return fmt.Errorf("mkdir data: %w", err)
	}

	flow, err := NewSimpleFlow(nm, 0, r.senderMode, dataDir)
	if err != nil {
		return fmt.Errorf("setup simple flow: %w", err)
	}
	defer func(flow Flow) {
		err := flow.Close()
		if err != nil {
			r.logger.Errorf("flow close: %v", err)
		}
	}(flow)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		err = flow.sender.Start(ctx)
		if err != nil {
			r.logger.Errorf("sender start: %v", err)
		}
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
	r.runNetworkSimulation(c, nm)
	return nil
}

func (r *Runner) runVariableAvailableCapacityMultipleFlows() error {
	nm, err := NewManager()
	if err != nil {
		return fmt.Errorf("new manager: %w", err)
	}

	dataDir := fmt.Sprintf("data/%v", r.name)
	err = os.MkdirAll(dataDir, os.ModePerm)
	if err != nil {
		return fmt.Errorf("mkdir data: %w", err)
	}

	for i := 0; i < 2; i++ {
		flow, err := NewSimpleFlow(nm, i, r.senderMode, dataDir)
		defer func(flow Flow) {
			err := flow.Close()
			if err != nil {
				r.logger.Errorf("flow close: %v", err)
			}
		}(flow)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go func() {
			err = flow.sender.Start(ctx)
			if err != nil {
				r.logger.Errorf("sender start: %v", err)
			}
		}()
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
	r.runNetworkSimulation(c, nm)
	return nil
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

func (r *Runner) runNetworkSimulation(c pathCharacteristics, nm *NetworkManager) {
	for _, phase := range c.phases {
		r.logger.Infof("enter next phase: %v\n", phase)
		nm.SetCapacity(
			int(float64(c.referenceCapacity)*phase.capacityRatio),
			phase.maxBurst,
		)
		time.Sleep(phase.duration)
	}
}
