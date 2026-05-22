package epg_test

import (
	"encoding/xml"
	"strings"
	"testing"
	"time"

	"github.com/e12media/satip-lab/internal/epg"
	"github.com/e12media/satip-lab/internal/lab"
)

type xmltv struct {
	XMLName    xml.Name       `xml:"tv"`
	Date       string         `xml:"date,attr"`
	Channels   []xmltvChannel `xml:"channel"`
	Programmes []xmltvProgram `xml:"programme"`
}

type xmltvChannel struct {
	ID          string `xml:"id,attr"`
	DisplayName string `xml:"display-name"`
}

type xmltvProgram struct {
	Channel string `xml:"channel,attr"`
	Start   string `xml:"start,attr"`
	Stop    string `xml:"stop,attr"`
	Title   string `xml:"title"`
	Episode string `xml:"episode-num"`
}

func TestParseClockDefaultsToFixedBerlinDSTInstant(t *testing.T) {
	clock, err := epg.ParseClock("")
	if err != nil {
		t.Fatal(err)
	}
	if clock.Mode != "fixed" {
		t.Fatalf("mode: got %q", clock.Mode)
	}
	if clock.Timezone != "Europe/Berlin" {
		t.Fatalf("timezone: got %q", clock.Timezone)
	}
	if got := clock.Now.Format(time.RFC3339); got != "2026-03-29T01:30:00+01:00" {
		t.Fatalf("default now: got %q", got)
	}
}

func TestGenerateXMLTVAlignsChannelIDsAndDisplayNamesWithCatalog(t *testing.T) {
	doc := generate(t, lab.Scenario{Name: lab.ScenarioNormal})

	want := map[string]string{
		"daserste.de": "Das Erste HD",
		"zdf.de":      "ZDF HD",
		"arte.de":     "arte HD",
		"3sat.de":     "3sat HD",
		"phoenix.de":  "phoenix HD",
	}
	if len(doc.Channels) != len(want) {
		t.Fatalf("channel count: got %d", len(doc.Channels))
	}
	for _, channel := range doc.Channels {
		if want[channel.ID] != channel.DisplayName {
			t.Fatalf("channel mismatch: id=%q display=%q", channel.ID, channel.DisplayName)
		}
		delete(want, channel.ID)
	}
	if len(want) != 0 {
		t.Fatalf("missing channels: %#v", want)
	}
}

func TestGenerateXMLTVUsesXMLTVTimeFormatAndCrossesDST(t *testing.T) {
	body, _ := generateBody(t, lab.Scenario{Name: lab.ScenarioNormal})
	if !strings.Contains(body, `start="20260329013000 +0100"`) {
		t.Fatalf("missing XMLTV start at lab clock: %s", body)
	}
	if !strings.Contains(body, " +0200") {
		t.Fatalf("expected EPG window to cross Europe/Berlin DST boundary: %s", body)
	}
	if strings.Contains(body, "2026-03-29T") {
		t.Fatalf("XMLTV body should not use RFC3339 timestamps: %s", body)
	}
}

func TestGenerateXMLTVUsesMixedScheduleDensity(t *testing.T) {
	doc := generate(t, lab.Scenario{Name: lab.ScenarioNormal})
	counts := programmeCounts(doc)
	if counts["zdf.de"] <= counts["phoenix.de"] {
		t.Fatalf("expected ZDF HD dense schedule and phoenix HD sparse schedule, got %#v", counts)
	}
	if counts["zdf.de"] < 40 {
		t.Fatalf("expected 30-minute ZDF HD programmes across 24h, got %#v", counts)
	}
	if counts["phoenix.de"] > 18 {
		t.Fatalf("expected sparse phoenix HD programmes, got %#v", counts)
	}
}

func TestGenerateXMLTVProvidesStableProgramIdentityBySlot(t *testing.T) {
	first, _ := generateBody(t, lab.Scenario{Name: lab.ScenarioNormal})
	second, _ := generateBody(t, lab.Scenario{Name: lab.ScenarioNormal})
	if first != second {
		t.Fatal("same fixed clock should produce identical XMLTV bytes")
	}

	doc := parseXMLTV(t, first)
	found := false
	for _, programme := range doc.Programmes {
		if programme.Channel == "zdf.de" && programme.Start == "20260329013000 +0100" {
			found = true
			if programme.Episode != "zdf.de-20260329013000" {
				t.Fatalf("stable programme id: got %q", programme.Episode)
			}
		}
	}
	if !found {
		t.Fatal("missing ZDF programme at lab clock")
	}
}

func TestEPGGapRemovesTargetedServiceWindow(t *testing.T) {
	doc := generate(t, lab.Scenario{Name: lab.ScenarioEPGGap, ServiceID: "arte-hd", DurationMin: 60})
	for _, programme := range doc.Programmes {
		if programme.Channel == "arte.de" && programme.Start == "20260329013000 +0100" {
			t.Fatalf("arte programme in targeted gap should be removed: %+v", programme)
		}
	}
	if programmeCounts(doc)["zdf.de"] == 0 {
		t.Fatal("targeted arte gap should not remove ZDF programmes")
	}
}

func TestEPGMismatchUsesUnknownXMLTVChannelID(t *testing.T) {
	doc := generate(t, lab.Scenario{Name: lab.ScenarioEPGMismatch})
	ids := map[string]bool{}
	for _, channel := range doc.Channels {
		ids[channel.ID] = true
	}
	if ids["zdf.de"] {
		t.Fatal("epg_mismatch should replace zdf.de with an unknown XMLTV id")
	}
	if !ids["zdf-mismatch.invalid"] {
		t.Fatalf("missing mismatch XMLTV id: %#v", ids)
	}
}

func TestEPGStaleLastModifiedIsRelativeToLabClock(t *testing.T) {
	clock, err := epg.ParseClock("")
	if err != nil {
		t.Fatal(err)
	}
	_, meta, err := epg.GenerateXMLTV(lab.DefaultCatalog(), clock, epg.Options{
		Scenario: lab.Scenario{Name: lab.ScenarioEPGStale},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := clock.Now.Add(-48 * time.Hour)
	if !meta.LastModified.Equal(want) {
		t.Fatalf("last modified: got %s want %s", meta.LastModified.Format(time.RFC3339), want.Format(time.RFC3339))
	}
}

func generate(t *testing.T, scenario lab.Scenario) xmltv {
	t.Helper()
	body, _ := generateBody(t, scenario)
	return parseXMLTV(t, body)
}

func generateBody(t *testing.T, scenario lab.Scenario) (string, epg.Metadata) {
	t.Helper()
	clock, err := epg.ParseClock("")
	if err != nil {
		t.Fatal(err)
	}
	body, meta, err := epg.GenerateXMLTV(lab.DefaultCatalog(), clock, epg.Options{Scenario: scenario})
	if err != nil {
		t.Fatal(err)
	}
	return string(body), meta
}

func parseXMLTV(t *testing.T, body string) xmltv {
	t.Helper()
	var doc xmltv
	if err := xml.Unmarshal([]byte(body), &doc); err != nil {
		t.Fatalf("invalid XMLTV: %v\n%s", err, body)
	}
	return doc
}

func programmeCounts(doc xmltv) map[string]int {
	counts := map[string]int{}
	for _, programme := range doc.Programmes {
		counts[programme.Channel]++
	}
	return counts
}
