package lab_test

import (
	"testing"
	"time"

	"github.com/e12media/satip-lab/internal/channels"
	"github.com/e12media/satip-lab/internal/lab"
)

func TestDefaultCatalogGroupsServicesByMux(t *testing.T) {
	catalog := lab.DefaultCatalog()

	if len(catalog.Services) != 5 {
		t.Fatalf("services: got %d", len(catalog.Services))
	}
	if len(catalog.Muxes) != 4 {
		t.Fatalf("muxes: got %d", len(catalog.Muxes))
	}

	dasErste, ok := catalog.ServiceByID("das-erste-hd")
	if !ok {
		t.Fatal("missing Das Erste HD service")
	}
	arte, ok := catalog.ServiceByID("arte-hd")
	if !ok {
		t.Fatal("missing arte HD service")
	}
	if dasErste.MuxID != arte.MuxID {
		t.Fatalf("expected services on 11494h to share mux: %s vs %s", dasErste.MuxID, arte.MuxID)
	}
	if dasErste.ServiceID == arte.ServiceID {
		t.Fatal("services on same mux need distinct service IDs")
	}
}

func TestCatalogFromChannelsUsesProvidedServices(t *testing.T) {
	catalog := lab.CatalogFromChannels([]channels.Channel{
		{ID: "custom-one", Number: 11, Name: "Custom One", Group: "Lab", TvgID: "custom-one.example", Frequency: 12000, Polarization: "h", SymbolRate: 27500, Delivery: "dvbs2", Src: 1, Pids: []int{0, 17, 4100, 4101, 4102}},
		{ID: "custom-two", Number: 12, Name: "Custom Two", Group: "Lab", TvgID: "custom-two.example", Frequency: 12000, Polarization: "h", SymbolRate: 27500, Delivery: "dvbs2", Src: 1, Pids: []int{0, 17, 4200, 4201, 4202}},
	})

	if len(catalog.Muxes) != 1 {
		t.Fatalf("muxes: got %+v", catalog.Muxes)
	}
	if len(catalog.Services) != 2 {
		t.Fatalf("services: got %+v", catalog.Services)
	}
	if catalog.Services[0].ID != "custom-one" || catalog.Services[0].PMTPID != 4100 {
		t.Fatalf("first service: %+v", catalog.Services[0])
	}
}

func TestManagerAllocatesAndSharesTunersByMux(t *testing.T) {
	manager := lab.NewManager(lab.DefaultCatalog(), 1)

	first, err := manager.Setup("sess-1", "src=1&freq=11494&pol=h&msys=dvbs2&sr=22000&pids=0,17,5100,5101,5102", "127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	if first.TunerID != 1 || first.Service.ID != "das-erste-hd" {
		t.Fatalf("unexpected first setup: %+v", first)
	}

	second, err := manager.Setup("sess-2", "src=1&freq=11494&pol=h&msys=dvbs2&sr=22000&pids=0,17,5200,5201,5202", "127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	if second.TunerID != first.TunerID || second.Service.ID != "arte-hd" {
		t.Fatalf("expected shared tuner on same mux: first=%+v second=%+v", first, second)
	}

	_, err = manager.Setup("sess-3", "src=1&freq=11362&pol=h&msys=dvbs2&sr=22000&pids=0,17,6100,6110,6120", "127.0.0.1")
	if err != lab.ErrNoTunerAvailable {
		t.Fatalf("expected tuner busy, got %v", err)
	}
}

func TestManagerReleasesTunerOnTeardown(t *testing.T) {
	manager := lab.NewManager(lab.DefaultCatalog(), 1)

	if _, err := manager.Setup("sess-1", "src=1&freq=11494&pol=h&msys=dvbs2&sr=22000&pids=0,17,5100,5101,5102", "127.0.0.1"); err != nil {
		t.Fatal(err)
	}
	manager.Teardown("sess-1")

	status := manager.Status()
	if status.Tuners[0].State != "idle" {
		t.Fatalf("expected idle tuner after teardown: %+v", status.Tuners[0])
	}

	if _, err := manager.Setup("sess-2", "src=1&freq=11362&pol=h&msys=dvbs2&sr=22000&pids=0,17,6100,6110,6120", "127.0.0.1"); err != nil {
		t.Fatalf("expected released tuner to accept different mux: %v", err)
	}
}

func TestManagerReportsLockedFrontendTelemetryAfterSetup(t *testing.T) {
	manager := lab.NewManager(lab.DefaultCatalog(), 1)

	if _, err := manager.Setup("sess-1", "src=1&freq=11494&pol=h&msys=dvbs2&sr=22000&pids=0,17,5100,5101,5102", "127.0.0.1"); err != nil {
		t.Fatal(err)
	}

	frontend := manager.Status().Tuners[0].Frontend
	if frontend.State != lab.FrontendLocked {
		t.Fatalf("frontend state: got %q", frontend.State)
	}
	if frontend.SignalStrength != 88 || frontend.SNRDB != 13.5 || frontend.BER != 0 || frontend.PER != 0 {
		t.Fatalf("locked telemetry: %+v", frontend)
	}
	if frontend.LockMS != 250 || frontend.LastLockChange == nil || frontend.LastLockChange.IsZero() {
		t.Fatalf("lock timing: %+v", frontend)
	}
}

func TestManagerResetsFrontendTelemetryWhenTunerIsReleased(t *testing.T) {
	manager := lab.NewManager(lab.DefaultCatalog(), 1)
	if _, err := manager.Setup("sess-1", "src=1&freq=11494&pol=h&msys=dvbs2&sr=22000&pids=0,17,5100,5101,5102", "127.0.0.1"); err != nil {
		t.Fatal(err)
	}

	manager.Teardown("sess-1")

	frontend := manager.Status().Tuners[0].Frontend
	if frontend.State != lab.FrontendIdle || frontend.SignalStrength != 0 || frontend.LockMS != 0 || frontend.LastLockChange != nil {
		t.Fatalf("released frontend telemetry: %+v", frontend)
	}
}

func TestSignalScenariosExposeDeterministicFrontendTelemetry(t *testing.T) {
	for _, tc := range []struct {
		scenario string
		state    string
		signal   int
		snr      float64
		ber      float64
		per      float64
		lockMS   int
	}{
		{scenario: lab.ScenarioSignalDegraded, state: lab.FrontendDegraded, signal: 42, snr: 6.5, ber: 0.00025, per: 0.02, lockMS: 250},
		{scenario: lab.ScenarioLockLoss, state: lab.FrontendLost, signal: 0, snr: 0, ber: 1, per: 1, lockMS: 250},
		{scenario: lab.ScenarioSlowLock, state: lab.FrontendTuning, signal: 55, snr: 8, ber: 0.0001, per: 0.01, lockMS: 1200},
	} {
		t.Run(tc.scenario, func(t *testing.T) {
			manager := lab.NewManager(lab.DefaultCatalog(), 1)
			if err := manager.SetScenario(tc.scenario); err != nil {
				t.Fatal(err)
			}
			if _, err := manager.Setup("sess-1", "src=1&freq=11494&pol=h&msys=dvbs2&sr=22000&pids=0,17,5100,5101,5102", "127.0.0.1"); err != nil {
				t.Fatal(err)
			}

			frontend := manager.Status().Tuners[0].Frontend
			if frontend.State != tc.state || frontend.SignalStrength != tc.signal || frontend.SNRDB != tc.snr || frontend.BER != tc.ber || frontend.PER != tc.per || frontend.LockMS != tc.lockMS {
				t.Fatalf("frontend telemetry for %s: got %+v", tc.scenario, frontend)
			}
		})
	}
}

func TestTargetedSignalScenarioOnlyChangesMatchingMuxTelemetry(t *testing.T) {
	manager := lab.NewManager(lab.DefaultCatalog(), 2)
	if err := manager.SetScenarioTarget(lab.ScenarioSignalDegraded, "", "src1-11362h-22000-dvbs2"); err != nil {
		t.Fatal(err)
	}

	if _, err := manager.Setup("sess-1", "src=1&freq=11494&pol=h&msys=dvbs2&sr=22000&pids=0,17,5100,5101,5102", "127.0.0.1"); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Setup("sess-2", "src=1&freq=11362&pol=h&msys=dvbs2&sr=22000&pids=0,17,6100,6110,6120", "127.0.0.1"); err != nil {
		t.Fatal(err)
	}

	status := manager.Status()
	if status.Tuners[0].Frontend.State != lab.FrontendLocked {
		t.Fatalf("non-targeted tuner should remain locked: %+v", status.Tuners[0])
	}
	if status.Tuners[1].Frontend.State != lab.FrontendDegraded {
		t.Fatalf("targeted tuner should be degraded: %+v", status.Tuners[1])
	}
}

func TestManagerTracksRequestedPIDs(t *testing.T) {
	manager := lab.NewManager(lab.DefaultCatalog(), 1)
	if _, err := manager.Setup("sess-1", "src=1&freq=11494&pol=h&msys=dvbs2&sr=22000&pids=0,17,5100,5101,5102", "127.0.0.1"); err != nil {
		t.Fatal(err)
	}
	if got := manager.Status().Sessions[0].PIDs; len(got) != 5 || got[2] != 5100 || got[4] != 5102 {
		t.Fatalf("initial PIDs: got %#v", got)
	}

	if err := manager.UpdatePIDs("sess-1", "addpids=5102&delpids=17"); err != nil {
		t.Fatal(err)
	}
	if got := manager.Status().Sessions[0].PIDs; !sameInts(got, []int{0, 5100, 5101, 5102}) {
		t.Fatalf("updated PIDs: got %#v", got)
	}
}

func TestManagerTracksAllPIDsMode(t *testing.T) {
	manager := lab.NewManager(lab.DefaultCatalog(), 1)
	if _, err := manager.Setup("sess-1", "src=1&freq=11494&pol=h&msys=dvbs2&sr=22000&pids=all", "127.0.0.1"); err != nil {
		t.Fatal(err)
	}
	if status := manager.Status().Sessions[0]; !status.PIDsAll || len(status.PIDs) != 0 {
		t.Fatalf("expected all PID mode, got %+v", status)
	}

	if err := manager.UpdatePIDs("sess-1", "pids=0,17,5100"); err != nil {
		t.Fatal(err)
	}
	if status := manager.Status().Sessions[0]; status.PIDsAll || !sameInts(status.PIDs, []int{0, 17, 5100}) {
		t.Fatalf("expected explicit PID mode, got %+v", status)
	}
}

func TestManagerRejectsInvalidPIDUpdates(t *testing.T) {
	manager := lab.NewManager(lab.DefaultCatalog(), 1)
	if _, err := manager.Setup("sess-1", "src=1&freq=11494&pol=h&msys=dvbs2&sr=22000&pids=0,17,5100,5101,5102", "127.0.0.1"); err != nil {
		t.Fatal(err)
	}

	if err := manager.UpdatePIDs("sess-1", "addpids=9000"); err != lab.ErrInvalidTune {
		t.Fatalf("out-of-range PID update: got %v", err)
	}
	if err := manager.UpdatePIDs("sess-1", "pids=abc"); err != lab.ErrInvalidTune {
		t.Fatalf("non-numeric PID update: got %v", err)
	}
	if got := manager.Status().Sessions[0].PIDs; !sameInts(got, []int{0, 17, 5100, 5101, 5102}) {
		t.Fatalf("invalid update should leave PIDs unchanged, got %#v", got)
	}
}

func TestManagerRejectsInvalidSetupPIDsWithoutAllocatingTuner(t *testing.T) {
	manager := lab.NewManager(lab.DefaultCatalog(), 1)

	_, err := manager.Setup("sess-1", "src=1&freq=11494&pol=h&msys=dvbs2&sr=22000&pids=0,17,5100,5101,5102,9000", "127.0.0.1")
	if err != lab.ErrInvalidTune {
		t.Fatalf("invalid setup PIDs: got %v", err)
	}
	if status := manager.Status(); len(status.Sessions) != 0 || status.Tuners[0].State != "idle" {
		t.Fatalf("invalid setup PIDs should not allocate state: %+v", status)
	}
}

func TestManagerRejectsExplicitDeleteFromAllPIDsMode(t *testing.T) {
	manager := lab.NewManager(lab.DefaultCatalog(), 1)
	if _, err := manager.Setup("sess-1", "src=1&freq=11494&pol=h&msys=dvbs2&sr=22000&pids=all", "127.0.0.1"); err != nil {
		t.Fatal(err)
	}

	if err := manager.UpdatePIDs("sess-1", "delpids=17"); err != lab.ErrInvalidTune {
		t.Fatalf("delpids from all mode: got %v", err)
	}
	if status := manager.Status().Sessions[0]; !status.PIDsAll || len(status.PIDs) != 0 {
		t.Fatalf("rejected delpids should keep all mode, got %+v", status)
	}
}

func TestManagerReportsInvalidTuning(t *testing.T) {
	manager := lab.NewManager(lab.DefaultCatalog(), 2)

	_, err := manager.Setup("bad", "src=1&freq=99999&pol=h&msys=dvbs2&sr=22000&pids=0,17,6100", "127.0.0.1")
	if err != lab.ErrServiceNotFound {
		t.Fatalf("expected service not found, got %v", err)
	}
}

func TestManagerScenarioDefaultsAndRejectsUnknownNames(t *testing.T) {
	manager := lab.NewManager(lab.DefaultCatalog(), 2)

	if got := manager.Scenario().Name; got != lab.ScenarioNormal {
		t.Fatalf("default scenario: got %q", got)
	}

	if err := manager.SetScenario("does-not-exist"); err == nil {
		t.Fatal("expected unknown scenario to be rejected")
	}
	if got := manager.Scenario().Name; got != lab.ScenarioNormal {
		t.Fatalf("scenario changed after rejected update: got %q", got)
	}
}

func TestManagerAcceptsVariantScenarioNames(t *testing.T) {
	manager := lab.NewManager(lab.DefaultCatalog(), 2)

	for _, name := range []string{
		lab.ScenarioBadM3U,
		lab.ScenarioTunerBusy,
		lab.ScenarioRTPStop,
		lab.ScenarioSlowRTSP,
		lab.ScenarioMalformedPSI,
		lab.ScenarioRTPLoss,
		lab.ScenarioRTPJitter,
		lab.ScenarioContinuityErrors,
		lab.ScenarioEPGGap,
		lab.ScenarioEPGMismatch,
		lab.ScenarioEPGStale,
		lab.ScenarioSignalDegraded,
		lab.ScenarioLockLoss,
		lab.ScenarioSlowLock,
	} {
		if err := manager.SetScenario(name); err != nil {
			t.Fatalf("SetScenario(%q): %v", name, err)
		}
		if got := manager.Scenario().Name; got != name {
			t.Fatalf("scenario after SetScenario(%q): got %q", name, got)
		}
	}
}

func TestEPGGapScenarioAcceptsTargetAndDuration(t *testing.T) {
	manager := lab.NewManager(lab.DefaultCatalog(), 2)

	if err := manager.SetScenarioOptions(lab.ScenarioEPGGap, "arte-hd", "", 90); err != nil {
		t.Fatal(err)
	}
	scenario := manager.Scenario()
	if scenario.Name != lab.ScenarioEPGGap || scenario.ServiceID != "arte-hd" || scenario.DurationMin != 90 {
		t.Fatalf("scenario: %+v", scenario)
	}
}

func TestDurationOnlyAppliesToEPGGap(t *testing.T) {
	manager := lab.NewManager(lab.DefaultCatalog(), 2)

	if err := manager.SetScenarioOptions(lab.ScenarioEPGStale, "", "", 90); err != lab.ErrScenarioDuration {
		t.Fatalf("epg_stale with duration: got %v", err)
	}
	if got := manager.Scenario().Name; got != lab.ScenarioNormal {
		t.Fatalf("rejected duration should not change scenario, got %q", got)
	}
}

func TestTunerBusyScenarioRejectsSetupWithoutAllocatingTuner(t *testing.T) {
	manager := lab.NewManager(lab.DefaultCatalog(), 1)
	if err := manager.SetScenario(lab.ScenarioTunerBusy); err != nil {
		t.Fatal(err)
	}

	_, err := manager.Setup("sess-1", "src=1&freq=11494&pol=h&msys=dvbs2&sr=22000&pids=0,17,5100,5101,5102", "127.0.0.1")
	if err != lab.ErrNoTunerAvailable {
		t.Fatalf("expected tuner busy, got %v", err)
	}

	status := manager.Status()
	if status.Tuners[0].State != "idle" || len(status.Sessions) != 0 {
		t.Fatalf("tuner_busy should not allocate state: %+v", status)
	}
}

func TestNoSignalScenarioRejectsSetupWithoutAllocatingTuner(t *testing.T) {
	manager := lab.NewManager(lab.DefaultCatalog(), 1)
	if err := manager.SetScenario(lab.ScenarioNoSignal); err != nil {
		t.Fatal(err)
	}

	_, err := manager.Setup("sess-1", "src=1&freq=11494&pol=h&msys=dvbs2&sr=22000&pids=0,17,5100,5101,5102", "127.0.0.1")
	if err != lab.ErrNoSignal {
		t.Fatalf("expected no signal, got %v", err)
	}

	status := manager.Status()
	if status.Tuners[0].State != "idle" || len(status.Sessions) != 0 {
		t.Fatalf("no_signal should not allocate state: %+v", status)
	}
}

func TestTargetedNoSignalScenarioOnlyAppliesToMatchingService(t *testing.T) {
	manager := lab.NewManager(lab.DefaultCatalog(), 1)
	if err := manager.SetScenarioTarget(lab.ScenarioNoSignal, "zdf-hd", ""); err != nil {
		t.Fatal(err)
	}

	if _, err := manager.Setup("sess-1", "src=1&freq=11494&pol=h&msys=dvbs2&sr=22000&pids=0,17,5100,5101,5102", "127.0.0.1"); err != nil {
		t.Fatalf("non-targeted service should still set up: %v", err)
	}

	_, err := manager.Setup("sess-2", "src=1&freq=11362&pol=h&msys=dvbs2&sr=22000&pids=0,17,6100,6110,6120", "127.0.0.1")
	if err != lab.ErrNoSignal {
		t.Fatalf("targeted service should return no signal, got %v", err)
	}
}

func TestTargetedNoSignalScenarioOnlyAppliesToMatchingMux(t *testing.T) {
	manager := lab.NewManager(lab.DefaultCatalog(), 1)
	if err := manager.SetScenarioTarget(lab.ScenarioNoSignal, "", "src1-11362h-22000-dvbs2"); err != nil {
		t.Fatal(err)
	}

	if _, err := manager.Setup("sess-1", "src=1&freq=11494&pol=h&msys=dvbs2&sr=22000&pids=0,17,5100,5101,5102", "127.0.0.1"); err != nil {
		t.Fatalf("non-targeted mux should still set up: %v", err)
	}

	_, err := manager.Setup("sess-2", "src=1&freq=11362&pol=h&msys=dvbs2&sr=22000&pids=0,17,6100,6110,6120", "127.0.0.1")
	if err != lab.ErrNoSignal {
		t.Fatalf("targeted mux should return no signal, got %v", err)
	}
}

func TestGlobalScenariosRejectTargets(t *testing.T) {
	manager := lab.NewManager(lab.DefaultCatalog(), 1)
	if err := manager.SetScenarioTarget(lab.ScenarioBadM3U, "zdf-hd", ""); err != lab.ErrScenarioTarget {
		t.Fatalf("bad_m3u with target: got %v", err)
	}
	if err := manager.SetScenarioTarget(lab.ScenarioSlowRTSP, "", "src1-11362h-22000-dvbs2"); err != lab.ErrScenarioTarget {
		t.Fatalf("slow_rtsp with target: got %v", err)
	}
	if got := manager.Scenario().Name; got != lab.ScenarioNormal {
		t.Fatalf("rejected target should not change scenario, got %q", got)
	}
}

func TestManagerExpiresIdleSessionsAndReleasesTuners(t *testing.T) {
	manager := lab.NewManager(lab.DefaultCatalog(), 1)
	if _, err := manager.Setup("sess-1", "src=1&freq=11494&pol=h&msys=dvbs2&sr=22000&pids=0,17,5100,5101,5102", "127.0.0.1"); err != nil {
		t.Fatal(err)
	}
	updatedAt := manager.Status().Sessions[0].UpdatedAt

	expired := manager.ExpireSessions(updatedAt.Add(61*time.Second), 60*time.Second)
	if len(expired) != 1 || expired[0] != "sess-1" {
		t.Fatalf("expired sessions: got %#v", expired)
	}
	status := manager.Status()
	if len(status.Sessions) != 0 || status.Tuners[0].State != "idle" {
		t.Fatalf("expired session should release state: %+v", status)
	}
}

func TestManagerTouchRefreshesSessionTimeout(t *testing.T) {
	manager := lab.NewManager(lab.DefaultCatalog(), 1)
	if _, err := manager.Setup("sess-1", "src=1&freq=11494&pol=h&msys=dvbs2&sr=22000&pids=0,17,5100,5101,5102", "127.0.0.1"); err != nil {
		t.Fatal(err)
	}
	updatedAt := manager.Status().Sessions[0].UpdatedAt
	if err := manager.Touch("sess-1", updatedAt.Add(30*time.Second)); err != nil {
		t.Fatal(err)
	}

	if expired := manager.ExpireSessions(updatedAt.Add(80*time.Second), 60*time.Second); len(expired) != 0 {
		t.Fatalf("session should still be active after touch, expired %#v", expired)
	}
	if expired := manager.ExpireSessions(updatedAt.Add(91*time.Second), 60*time.Second); len(expired) != 1 {
		t.Fatalf("session should expire after refreshed timeout, expired %#v", expired)
	}
}

func sameInts(got, want []int) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}
