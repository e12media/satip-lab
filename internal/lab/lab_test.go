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

func TestManagerReportsFrontendLifecycleAfterSetup(t *testing.T) {
	manager := lab.NewManager(lab.DefaultCatalog(), 1)

	setup, err := manager.Setup("sess-1", "src=1&freq=11494&pol=h&msys=dvbs2&sr=22000&pids=0,17,5100,5101,5102", "127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}

	frontend := manager.StatusAt(setup.Session.CreatedAt).Tuners[0].Frontend
	if frontend.State != lab.FrontendTuning {
		t.Fatalf("frontend should tune before lock acquisition, got %q", frontend.State)
	}
	if frontend.LockMS != 250 || frontend.LastLockChange == nil || !frontend.LastLockChange.Equal(setup.Session.CreatedAt) {
		t.Fatalf("tuning telemetry: %+v start=%s", frontend, setup.Session.CreatedAt)
	}

	frontend = manager.StatusAt(setup.Session.CreatedAt.Add(250 * time.Millisecond)).Tuners[0].Frontend
	if frontend.State != lab.FrontendLocked {
		t.Fatalf("frontend state: got %q", frontend.State)
	}
	if frontend.SignalStrength != 88 || frontend.SNRDB != 13.5 || frontend.BER != 0 || frontend.PER != 0 {
		t.Fatalf("locked telemetry: %+v", frontend)
	}
	if frontend.LockMS != 250 || frontend.LastLockChange == nil || !frontend.LastLockChange.Equal(setup.Session.CreatedAt.Add(250*time.Millisecond)) {
		t.Fatalf("lock timing: %+v", frontend)
	}
}

func TestSameMuxSharingPreservesFrontendLifecycle(t *testing.T) {
	manager := lab.NewManager(lab.DefaultCatalog(), 1)

	first, err := manager.Setup("sess-1", "src=1&freq=11494&pol=h&msys=dvbs2&sr=22000&pids=0,17,5100,5101,5102", "127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	second, err := manager.Setup("sess-2", "src=1&freq=11494&pol=h&msys=dvbs2&sr=22000&pids=0,17,5200,5201,5202", "127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	if second.TunerID != first.TunerID {
		t.Fatalf("expected same mux to share tuner: first=%d second=%d", first.TunerID, second.TunerID)
	}

	frontend := manager.StatusAt(first.Session.CreatedAt.Add(100 * time.Millisecond)).Tuners[0].Frontend
	if frontend.State != lab.FrontendTuning {
		t.Fatalf("same-mux sharing should not reset or skip frontend lifecycle: %+v", frontend)
	}
	if frontend.LastLockChange == nil || !frontend.LastLockChange.Equal(first.Session.CreatedAt) {
		t.Fatalf("same-mux sharing should preserve first tune start: %+v first=%s second=%s", frontend, first.Session.CreatedAt, second.Session.CreatedAt)
	}
}

func TestTimelineLockLossRecoversFrontendLifecycle(t *testing.T) {
	manager := lab.NewManager(lab.DefaultCatalog(), 1)
	setup, err := manager.Setup("sess-1", "src=1&freq=11494&pol=h&msys=dvbs2&sr=22000&pids=0,17,5100,5101,5102", "127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	start := setup.Session.CreatedAt.Add(250 * time.Millisecond)
	if err := manager.SetScenarioTimelineAt([]lab.ScenarioTimelineStep{
		{AtMS: 0, Name: lab.ScenarioLockLoss},
		{AtMS: 1000, Name: lab.ScenarioNormal},
	}, start); err != nil {
		t.Fatal(err)
	}

	if got := manager.StatusAt(start.Add(500 * time.Millisecond)).Tuners[0].Frontend.State; got != lab.FrontendLost {
		t.Fatalf("timeline lock loss state: got %q", got)
	}
	recovering := manager.StatusAt(start.Add(1100 * time.Millisecond)).Tuners[0].Frontend
	if recovering.State != lab.FrontendRecovering {
		t.Fatalf("expected recovering after lock_loss clears, got %+v", recovering)
	}
	if recovering.LastLockChange == nil || !recovering.LastLockChange.Equal(start.Add(1000*time.Millisecond)) {
		t.Fatalf("recovering lock timing: %+v", recovering)
	}
	locked := manager.StatusAt(start.Add(1300 * time.Millisecond)).Tuners[0].Frontend
	if locked.State != lab.FrontendLocked {
		t.Fatalf("expected locked after recovery window, got %q", locked.State)
	}
	if locked.LastLockChange == nil || !locked.LastLockChange.Equal(start.Add(1250*time.Millisecond)) {
		t.Fatalf("locked recovery timing: %+v", locked)
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

	first, err := manager.Setup("sess-1", "src=1&freq=11494&pol=h&msys=dvbs2&sr=22000&pids=0,17,5100,5101,5102", "127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Setup("sess-2", "src=1&freq=11362&pol=h&msys=dvbs2&sr=22000&pids=0,17,6100,6110,6120", "127.0.0.1"); err != nil {
		t.Fatal(err)
	}

	status := manager.StatusAt(first.Session.CreatedAt.Add(250 * time.Millisecond))
	if status.Tuners[0].Frontend.State != lab.FrontendLocked {
		t.Fatalf("non-targeted tuner should remain locked: %+v", status.Tuners[0])
	}
	if status.Tuners[1].Frontend.State != lab.FrontendDegraded {
		t.Fatalf("targeted tuner should be degraded: %+v", status.Tuners[1])
	}
}

func TestRuntimeSignalScenarioUpdatesActiveTunerTelemetry(t *testing.T) {
	manager := lab.NewManager(lab.DefaultCatalog(), 1)
	setup, err := manager.Setup("sess-1", "src=1&freq=11494&pol=h&msys=dvbs2&sr=22000&pids=0,17,5100,5101,5102", "127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	if got := manager.StatusAt(setup.Session.CreatedAt.Add(250 * time.Millisecond)).Tuners[0].Frontend.State; got != lab.FrontendLocked {
		t.Fatalf("initial frontend state: got %q", got)
	}

	if err := manager.SetScenario(lab.ScenarioSignalDegraded); err != nil {
		t.Fatal(err)
	}
	if got := manager.Status().Tuners[0].Frontend.State; got != lab.FrontendDegraded {
		t.Fatalf("scenario change should update active tuner telemetry, got %q", got)
	}

	if err := manager.SetScenario(lab.ScenarioNormal); err != nil {
		t.Fatal(err)
	}
	if got := manager.StatusAt(setup.Session.CreatedAt.Add(250 * time.Millisecond)).Tuners[0].Frontend.State; got != lab.FrontendLocked {
		t.Fatalf("restoring normal should update active tuner telemetry, got %q", got)
	}
}

func TestServiceTargetedTelemetryIsStableForSameMuxSharing(t *testing.T) {
	manager := lab.NewManager(lab.DefaultCatalog(), 1)
	if err := manager.SetScenarioTarget(lab.ScenarioSignalDegraded, "arte-hd", ""); err != nil {
		t.Fatal(err)
	}
	setup, err := manager.Setup("sess-arte", "src=1&freq=11494&pol=h&msys=dvbs2&sr=22000&pids=0,17,5200,5201,5202", "127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Setup("sess-daserste", "src=1&freq=11494&pol=h&msys=dvbs2&sr=22000&pids=0,17,5100,5101,5102", "127.0.0.1"); err != nil {
		t.Fatal(err)
	}

	status := manager.Status()
	if status.Tuners[0].Frontend.State != lab.FrontendDegraded {
		t.Fatalf("shared tuner should stay degraded while targeted service is active: %+v", status.Tuners[0])
	}

	manager.Teardown("sess-arte")
	status = manager.StatusAt(setup.Session.CreatedAt.Add(250 * time.Millisecond))
	if status.Tuners[0].Frontend.State != lab.FrontendLocked {
		t.Fatalf("shared tuner should return locked after targeted service leaves: %+v", status.Tuners[0])
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

func TestScenarioTimelineAdvancesByElapsedTime(t *testing.T) {
	manager := lab.NewManager(lab.DefaultCatalog(), 2)
	start := time.Date(2026, 6, 9, 12, 0, 0, 0, time.UTC)
	steps := []lab.ScenarioTimelineStep{
		{AtMS: 0, Name: lab.ScenarioNormal},
		{AtMS: 1000, Name: lab.ScenarioSignalDegraded, MuxID: "src1-11362h-22000-dvbs2"},
		{AtMS: 2500, Name: lab.ScenarioLockLoss, MuxID: "src1-11362h-22000-dvbs2"},
	}

	if err := manager.SetScenarioTimelineAt(steps, start); err != nil {
		t.Fatal(err)
	}
	if got := manager.ScenarioAt(start.Add(500 * time.Millisecond)); got.Name != lab.ScenarioNormal || got.Timeline == nil || got.Timeline.StepIndex != 0 {
		t.Fatalf("timeline before first transition: %+v", got)
	}
	middle := manager.ScenarioAt(start.Add(1500 * time.Millisecond))
	if middle.Name != lab.ScenarioSignalDegraded || middle.MuxID != "src1-11362h-22000-dvbs2" || middle.Timeline == nil || middle.Timeline.StepIndex != 1 {
		t.Fatalf("timeline after degraded transition: %+v", middle)
	}
	if got := manager.ScenarioAt(start.Add(3 * time.Second)); got.Name != lab.ScenarioLockLoss || got.Timeline == nil || got.Timeline.StepIndex != 2 || got.Timeline.ElapsedMS != 3000 {
		t.Fatalf("timeline after lock loss transition: %+v", got)
	}
}

func TestScenarioTimelineRecordsTransitionEvents(t *testing.T) {
	manager := lab.NewManager(lab.DefaultCatalog(), 2)
	start := time.Date(2026, 6, 9, 12, 0, 0, 0, time.UTC)
	if err := manager.SetScenarioTimelineAt([]lab.ScenarioTimelineStep{
		{AtMS: 0, Name: lab.ScenarioNormal},
		{AtMS: 1000, Name: lab.ScenarioSignalDegraded},
	}, start); err != nil {
		t.Fatal(err)
	}

	_ = manager.ScenarioAt(start.Add(1500 * time.Millisecond))

	events := manager.Status().Events
	if len(events) < 2 {
		t.Fatalf("expected timeline events, got %+v", events)
	}
	if events[len(events)-1].Type != "scenario_timeline_step" || events[len(events)-1].Message != lab.ScenarioSignalDegraded {
		t.Fatalf("expected transition event, got %+v", events)
	}
}

func TestStatusAdvancesTimelineTelemetry(t *testing.T) {
	manager := lab.NewManager(lab.DefaultCatalog(), 1)
	start := time.Date(2026, 6, 9, 12, 0, 0, 0, time.UTC)
	if err := manager.SetScenarioTimelineAt([]lab.ScenarioTimelineStep{
		{AtMS: 0, Name: lab.ScenarioNormal},
		{AtMS: 1000, Name: lab.ScenarioSignalDegraded},
	}, start); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Setup("sess-1", "src=1&freq=11494&pol=h&msys=dvbs2&sr=22000&pids=0,17,5100,5101,5102", "127.0.0.1"); err != nil {
		t.Fatal(err)
	}
	if got := manager.StatusAt(start.Add(1500 * time.Millisecond)).Tuners[0].Frontend.State; got != lab.FrontendDegraded {
		t.Fatalf("status should advance timeline telemetry, got %q", got)
	}
}

func TestScenarioTimelineAppliesInitialTelemetryToActiveTuners(t *testing.T) {
	manager := lab.NewManager(lab.DefaultCatalog(), 1)
	start := time.Date(2026, 6, 9, 12, 0, 0, 0, time.UTC)
	if _, err := manager.Setup("sess-1", "src=1&freq=11494&pol=h&msys=dvbs2&sr=22000&pids=0,17,5100,5101,5102", "127.0.0.1"); err != nil {
		t.Fatal(err)
	}
	if err := manager.SetScenarioTimelineAt([]lab.ScenarioTimelineStep{
		{AtMS: 0, Name: lab.ScenarioSignalDegraded},
		{AtMS: 1000, Name: lab.ScenarioNormal},
	}, start); err != nil {
		t.Fatal(err)
	}

	if got := manager.StatusAt(start).Tuners[0].Frontend.State; got != lab.FrontendDegraded {
		t.Fatalf("initial timeline step should update telemetry, got %q", got)
	}
}

func TestScenarioTimelineRejectsInvalidSteps(t *testing.T) {
	manager := lab.NewManager(lab.DefaultCatalog(), 2)
	start := time.Date(2026, 6, 9, 12, 0, 0, 0, time.UTC)

	if err := manager.SetScenarioTimelineAt(nil, start); err != lab.ErrScenarioTimeline {
		t.Fatalf("empty timeline: got %v", err)
	}
	if err := manager.SetScenarioTimelineAt([]lab.ScenarioTimelineStep{{AtMS: 10, Name: lab.ScenarioNormal}}, start); err != lab.ErrScenarioTimeline {
		t.Fatalf("timeline without zero start: got %v", err)
	}
	if err := manager.SetScenarioTimelineAt([]lab.ScenarioTimelineStep{{AtMS: 0, Name: lab.ScenarioNormal}, {AtMS: 0, Name: lab.ScenarioRTPStop}}, start); err != lab.ErrScenarioTimeline {
		t.Fatalf("duplicate at_ms: got %v", err)
	}
	if err := manager.SetScenarioTimelineAt([]lab.ScenarioTimelineStep{{AtMS: 0, Name: lab.ScenarioNormal}, {AtMS: 500, Name: "missing"}}, start); err != lab.ErrUnknownScenario {
		t.Fatalf("unknown scenario step: got %v", err)
	}
	if err := manager.SetScenarioTimelineAt([]lab.ScenarioTimelineStep{{AtMS: 0, Name: lab.ScenarioNormal}, {AtMS: 500, Name: lab.ScenarioBadM3U, ServiceID: "zdf-hd"}}, start); err != lab.ErrScenarioTarget {
		t.Fatalf("global scenario target: got %v", err)
	}
}

func TestSetScenarioClearsActiveTimeline(t *testing.T) {
	manager := lab.NewManager(lab.DefaultCatalog(), 2)
	start := time.Date(2026, 6, 9, 12, 0, 0, 0, time.UTC)
	if err := manager.SetScenarioTimelineAt([]lab.ScenarioTimelineStep{
		{AtMS: 0, Name: lab.ScenarioNormal},
		{AtMS: 1000, Name: lab.ScenarioSignalDegraded},
	}, start); err != nil {
		t.Fatal(err)
	}

	if err := manager.SetScenario(lab.ScenarioRTPStop); err != nil {
		t.Fatal(err)
	}

	got := manager.ScenarioAt(start.Add(1500 * time.Millisecond))
	if got.Name != lab.ScenarioRTPStop || got.Timeline != nil {
		t.Fatalf("explicit scenario should clear timeline, got %+v", got)
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

func TestTunerWedgedScenarioRejectsSetupUntilReset(t *testing.T) {
	manager := lab.NewManager(lab.DefaultCatalog(), 1)
	if err := manager.SetScenario(lab.ScenarioTunerWedged); err != nil {
		t.Fatal(err)
	}

	_, err := manager.Setup("sess-1", "src=1&freq=11494&pol=h&msys=dvbs2&sr=22000&pids=0,17,5100,5101,5102", "127.0.0.1")
	if err != lab.ErrTunerWedged {
		t.Fatalf("expected tuner wedged, got %v", err)
	}

	manager.Reset()
	if got := manager.Scenario().Name; got != lab.ScenarioNormal {
		t.Fatalf("reset should clear wedged scenario, got %q", got)
	}
	if _, err := manager.Setup("sess-2", "src=1&freq=11494&pol=h&msys=dvbs2&sr=22000&pids=0,17,5100,5101,5102", "127.0.0.1"); err != nil {
		t.Fatalf("setup after reset: %v", err)
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

func TestSignalRecoveryScenarioReportsRecoveringThenLocked(t *testing.T) {
	manager := lab.NewManager(lab.DefaultCatalog(), 1)
	if err := manager.SetScenario(lab.ScenarioSignalRecovery); err != nil {
		t.Fatal(err)
	}
	setup, err := manager.Setup("sess-1", "src=1&freq=11494&pol=h&msys=dvbs2&sr=22000&pids=0,17,5100,5101,5102", "127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}

	recovering := manager.StatusAt(setup.Session.CreatedAt).Tuners[0].Frontend
	if recovering.State != lab.FrontendRecovering {
		t.Fatalf("expected recovering immediately after setup, got %+v", recovering)
	}
	locked := manager.StatusAt(setup.Session.CreatedAt.Add(250 * time.Millisecond)).Tuners[0].Frontend
	if locked.State != lab.FrontendLocked || locked.LastLockChange == nil || !locked.LastLockChange.Equal(setup.Session.CreatedAt.Add(250*time.Millisecond)) {
		t.Fatalf("expected locked after recovery window, got %+v", locked)
	}
}

func TestTimelineSignalRecoveryAfterLockLoss(t *testing.T) {
	manager := lab.NewManager(lab.DefaultCatalog(), 1)
	setup, err := manager.Setup("sess-1", "src=1&freq=11494&pol=h&msys=dvbs2&sr=22000&pids=0,17,5100,5101,5102", "127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	start := setup.Session.CreatedAt.Add(250 * time.Millisecond)
	if err := manager.SetScenarioTimelineAt([]lab.ScenarioTimelineStep{
		{AtMS: 0, Name: lab.ScenarioLockLoss},
		{AtMS: 1000, Name: lab.ScenarioSignalRecovery},
	}, start); err != nil {
		t.Fatal(err)
	}

	if got := manager.StatusAt(start.Add(500 * time.Millisecond)).Tuners[0].Frontend.State; got != lab.FrontendLost {
		t.Fatalf("lock loss state: got %q", got)
	}
	if got := manager.StatusAt(start.Add(1100 * time.Millisecond)).Tuners[0].Frontend.State; got != lab.FrontendRecovering {
		t.Fatalf("recovery state: got %q", got)
	}
	if got := manager.StatusAt(start.Add(1300 * time.Millisecond)).Tuners[0].Frontend.State; got != lab.FrontendLocked {
		t.Fatalf("post-recovery state: got %q", got)
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
