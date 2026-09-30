package ackhandler

import (
	"reflect"
	"sync"
	"testing"

	"github.com/poise52/quic-go/congestion"
	"github.com/poise52/quic-go/internal/monotime"
	"github.com/poise52/quic-go/internal/protocol"
	"github.com/poise52/quic-go/internal/utils"
	"github.com/stretchr/testify/require"
)

type factoryTestController struct {
	generation int
}

func (*factoryTestController) SetRTTStatsProvider(congestion.RTTStatsProvider) {}
func (*factoryTestController) TimeUntilSend(congestion.ByteCount) monotime.Time {
	return monotime.Time(0)
}
func (*factoryTestController) HasPacingBudget(monotime.Time) bool { return true }
func (*factoryTestController) OnPacketSent(monotime.Time, congestion.ByteCount, congestion.PacketNumber, congestion.ByteCount, bool) {
}
func (*factoryTestController) CanSend(congestion.ByteCount) bool { return true }
func (*factoryTestController) MaybeExitSlowStart()               {}
func (*factoryTestController) OnPacketAcked(congestion.PacketNumber, congestion.ByteCount, congestion.ByteCount, monotime.Time) {
}
func (*factoryTestController) OnCongestionEvent(congestion.PacketNumber, congestion.ByteCount, congestion.ByteCount) {
}
func (*factoryTestController) OnRetransmissionTimeout(bool)              {}
func (*factoryTestController) SetMaxDatagramSize(congestion.ByteCount)   {}
func (*factoryTestController) InSlowStart() bool                         { return false }
func (*factoryTestController) InRecovery() bool                          { return false }
func (*factoryTestController) GetCongestionWindow() congestion.ByteCount { return 0 }

func newCongestionTestHandler(t *testing.T) *sentPacketHandler {
	t.Helper()
	return NewSentPacketHandler(
		0, 1200, utils.NewRTTStats(), &utils.ConnectionStats{}, true, false, nil,
		protocol.PerspectiveClient, false, nil, utils.DefaultLogger,
	).(*sentPacketHandler)
}

func TestCubicAndRenoControllersSurvivePathMigration(t *testing.T) {
	for _, tc := range []struct {
		name string
		reno bool
	}{{name: "cubic"}, {name: "new-reno", reno: true}} {
		t.Run(tc.name, func(t *testing.T) {
			h := newCongestionTestHandler(t)
			h.SetCubicCongestionControl(tc.reno)
			require.Equal(t, tc.reno, cubicUsesReno(h))

			h.MigratedPath(monotime.Now(), 1350)
			require.Equal(t, tc.reno, cubicUsesReno(h))
			require.Equal(t, protocol.ByteCount(1350), h.maxDatagramSize)
		})
	}
}

func cubicUsesReno(h *sentPacketHandler) bool {
	return reflect.ValueOf(h.congestion).Elem().FieldByName("reno").Bool()
}

func TestCustomCongestionFactorySurvivesPathMigration(t *testing.T) {
	h := newCongestionTestHandler(t)
	var sizes []congestion.ByteCount
	h.SetCongestionControlFactory(func(size congestion.ByteCount) congestion.CongestionControl {
		sizes = append(sizes, size)
		return &factoryTestController{generation: len(sizes)}
	})
	first := h.congestion.(*ccAdapter).CC
	require.IsType(t, &factoryTestController{}, first)

	h.MigratedPath(monotime.Now(), 1400)
	second := h.congestion.(*ccAdapter).CC
	require.IsType(t, &factoryTestController{}, second)
	require.NotSame(t, first, second)
	require.Equal(t, []congestion.ByteCount{1200, 1400}, sizes)
}

func TestControllerFactoryAndPMTUUpdatesAreSynchronized(t *testing.T) {
	h := newCongestionTestHandler(t)
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := range 1000 {
			h.SetCubicCongestionControl(i%2 == 0)
		}
	}()
	go func() {
		defer wg.Done()
		for i := range 300 {
			h.SetMaxDatagramSize(protocol.ByteCount(1201 + i))
		}
	}()
	wg.Wait()
}
