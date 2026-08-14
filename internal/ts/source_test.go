package ts_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/e12media/satip-lab/internal/ts"
)

func TestSampleProfileUsesBundledPayloadOnlyForZDF(t *testing.T) {
	dir := t.TempDir()
	writeSample(t, dir, "h264_aac_short.ts", "H264-AAC-SAMPLE")

	source := ts.Source{SampleProfile: ts.SampleProfileH264AACShort, SampleAssetDir: dir}

	zdf, err := source.LoadServicePayload(ts.ServiceProfile{ID: "zdf-hd", Name: "ZDF HD"})
	if err != nil {
		t.Fatal(err)
	}
	if string(zdf) != "H264-AAC-SAMPLE" {
		t.Fatalf("zdf payload: got %q", string(zdf))
	}

	dasErste, err := source.LoadServicePayload(ts.ServiceProfile{ID: "das-erste-hd", Name: "Das Erste HD"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(dasErste), "H264-AAC-SAMPLE") {
		t.Fatalf("non-ZDF service should remain synthetic: %q", string(dasErste))
	}
	if !strings.Contains(string(dasErste), "das-erste-hd") {
		t.Fatalf("missing synthetic service marker: %q", string(dasErste))
	}
}

func TestLoadServicePayloadWithOptionsUsesEITClock(t *testing.T) {
	source := ts.Source{}
	loc, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		t.Fatal(err)
	}
	payload, err := source.LoadServicePayloadWithOptions(ts.ServiceProfile{
		ID:        "das-erste-hd",
		Name:      "Das Erste HD",
		ServiceID: 1001,
		PMTPID:    5100,
		VideoPID:  5101,
		AudioPID:  5102,
	}, ts.EITOptions{Now: time.Date(2026, 3, 29, 4, 45, 0, 0, loc)})
	if err != nil {
		t.Fatal(err)
	}
	sections := sectionsByPID(payload, 0x12)
	if len(sections) != 2 {
		t.Fatalf("EIT sections: got %d", len(sections))
	}
	if got := sections[0][18:21]; !bytes.Equal(got, []byte{0x02, 0x45, 0x00}) {
		t.Fatalf("present EIT start should use configured UTC clock, got % x", got)
	}
}

func TestLoadServicePayloadWithOptionsCanSuppressEIT(t *testing.T) {
	source := ts.Source{}
	payload, err := source.LoadServicePayloadWithOptions(ts.ServiceProfile{
		ID:        "arte-hd",
		Name:      "arte HD",
		ServiceID: 1003,
		PMTPID:    5200,
		VideoPID:  5201,
		AudioPID:  5202,
	}, ts.EITOptions{Suppress: true})
	if err != nil {
		t.Fatal(err)
	}
	if sections := sectionsByPID(payload, 0x12); len(sections) != 0 {
		t.Fatalf("suppressed EIT sections: got %d", len(sections))
	}
	if !strings.Contains(string(payload), "arte-hd") {
		t.Fatal("synthetic media markers should remain when EIT is suppressed")
	}
}

func TestSilentSampleProfileUsesSilentAsset(t *testing.T) {
	dir := t.TempDir()
	writeSample(t, dir, "h264_silent.ts", "H264-SILENT-SAMPLE")

	source := ts.Source{SampleProfile: ts.SampleProfileH264Silent, SampleAssetDir: dir}

	payload, err := source.LoadServicePayload(ts.ServiceProfile{ID: "zdf-hd", Name: "ZDF HD"})
	if err != nil {
		t.Fatal(err)
	}
	if string(payload) != "H264-SILENT-SAMPLE" {
		t.Fatalf("payload: got %q", string(payload))
	}
}

func TestMediaDirUsesPerServicePayload(t *testing.T) {
	dir := t.TempDir()
	writeSample(t, dir, "das-erste-hd.ts", "DAS-ERSTE-MEDIA")

	source := ts.Source{MediaDir: dir}

	dasErste, err := source.LoadServicePayload(ts.ServiceProfile{ID: "das-erste-hd", Name: "Das Erste HD"})
	if err != nil {
		t.Fatal(err)
	}
	if string(dasErste) != "DAS-ERSTE-MEDIA" {
		t.Fatalf("media payload: got %q", string(dasErste))
	}

	zdf, err := source.LoadServicePayload(ts.ServiceProfile{ID: "zdf-hd", Name: "ZDF HD"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(zdf), "DAS-ERSTE-MEDIA") {
		t.Fatalf("missing service-specific media should not reuse another service asset: %q", string(zdf))
	}
	if !strings.Contains(string(zdf), "zdf-hd") {
		t.Fatalf("missing media asset should fall back to synthetic payload: %q", string(zdf))
	}
}

func TestMediaDirOverridesSampleProfile(t *testing.T) {
	dir := t.TempDir()
	sampleDir := t.TempDir()
	writeSample(t, dir, "zdf-hd.ts", "ZDF-MEDIA")
	writeSample(t, sampleDir, "h264_aac_short.ts", "H264-AAC-SAMPLE")

	source := ts.Source{MediaDir: dir, SampleProfile: ts.SampleProfileH264AACShort, SampleAssetDir: sampleDir}

	payload, err := source.LoadServicePayload(ts.ServiceProfile{ID: "zdf-hd", Name: "ZDF HD"})
	if err != nil {
		t.Fatal(err)
	}
	if string(payload) != "ZDF-MEDIA" {
		t.Fatalf("media dir should override sample profile, got %q", string(payload))
	}
}

func TestMediaDirUsesBasenameForServiceID(t *testing.T) {
	dir := t.TempDir()
	writeSample(t, dir, "zdf-hd.ts", "SAFE-MEDIA")

	source := ts.Source{MediaDir: dir}

	payload, err := source.LoadServicePayload(ts.ServiceProfile{ID: "../zdf-hd", Name: "ZDF HD"})
	if err != nil {
		t.Fatal(err)
	}
	if string(payload) != "SAFE-MEDIA" {
		t.Fatalf("media filename should use basename, got %q", string(payload))
	}
}

func TestTransportStreamPathOverridesSampleProfile(t *testing.T) {
	dir := t.TempDir()
	globalPath := filepath.Join(dir, "global.ts")
	if err := os.WriteFile(globalPath, []byte("GLOBAL-FILE"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeSample(t, dir, "h264_aac_short.ts", "H264-AAC-SAMPLE")

	writeSample(t, dir, "zdf-hd.ts", "ZDF-MEDIA")

	source := ts.Source{Path: globalPath, MediaDir: dir, SampleProfile: ts.SampleProfileH264AACShort, SampleAssetDir: dir}

	for _, serviceID := range []string{"zdf-hd", "das-erste-hd"} {
		payload, err := source.LoadServicePayload(ts.ServiceProfile{ID: serviceID})
		if err != nil {
			t.Fatal(err)
		}
		if string(payload) != "GLOBAL-FILE" {
			t.Fatalf("%s payload: got %q", serviceID, string(payload))
		}
	}
}

func TestTransportStreamPathOverridesEITOptions(t *testing.T) {
	dir := t.TempDir()
	globalPath := filepath.Join(dir, "global.ts")
	if err := os.WriteFile(globalPath, []byte("GLOBAL-FILE"), 0o644); err != nil {
		t.Fatal(err)
	}
	source := ts.Source{Path: globalPath}

	payload, err := source.LoadServicePayloadWithOptions(ts.ServiceProfile{ID: "das-erste-hd"}, ts.EITOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if string(payload) != "GLOBAL-FILE" {
		t.Fatalf("global TS path should remain untouched, got %q", string(payload))
	}
}

func TestChunkAtWrapsOffsetOutsidePayload(t *testing.T) {
	source := ts.Source{}
	payload := []byte("short-payload")

	chunk, next := source.ChunkAt(payload, len(payload)+100)

	if !bytes.Equal(chunk, payload) {
		t.Fatalf("chunk: got %q", string(chunk))
	}
	if next != 0 {
		t.Fatalf("next offset: got %d", next)
	}
}

func TestEnabledSampleProfileReturnsErrorWhenAssetMissing(t *testing.T) {
	source := ts.Source{SampleProfile: ts.SampleProfileH264AACShort, SampleAssetDir: t.TempDir()}

	_, err := source.LoadServicePayload(ts.ServiceProfile{ID: "zdf-hd"})
	if err == nil {
		t.Fatal("expected missing sample asset to return an error")
	}
}

func TestMediaDirReturnsErrorForEmptyServiceAsset(t *testing.T) {
	dir := t.TempDir()
	writeSample(t, dir, "zdf-hd.ts", "")

	source := ts.Source{MediaDir: dir}

	_, err := source.LoadServicePayload(ts.ServiceProfile{ID: "zdf-hd"})
	if err == nil {
		t.Fatal("expected empty service media asset to return an error")
	}
}

func writeSample(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}
