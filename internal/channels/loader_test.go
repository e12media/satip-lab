package channels_test

import (
	"os"
	"strings"
	"testing"

	"github.com/e12media/satip-lab/internal/channels"
)

func TestLoadCatalogFileParsesYAMLServices(t *testing.T) {
	path := writeCatalogFixture(t, `services:
  - id: test-one-hd
    number: 101
    name: Test One HD
    group: Test
    tvg_id: test-one.example
    src: 1
    freq: 11494
    pol: h
    sr: 22000
    msys: dvbs2
    pids: [0, 17, 7100, 7101, 7102]
  - id: test-two-hd
    number: 102
    name: Test Two HD
    group: Test
    tvg_id: test-two.example
    src: 1
    freq: 11362
    pol: v
    sr: 27500
    msys: dvbs
    pids: [0, 17, 7200, 7201, 7202]
`)

	got, err := channels.LoadCatalogFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("services: got %d", len(got))
	}
	if got[0].ID != "test-one-hd" || got[0].TvgID != "test-one.example" {
		t.Fatalf("first service: %+v", got[0])
	}
	if got[1].Frequency != 11362 || got[1].Delivery != "dvbs" || got[1].Pids[4] != 7202 {
		t.Fatalf("second service: %+v", got[1])
	}
}

func TestLoadCatalogFileRejectsInvalidCatalog(t *testing.T) {
	path := writeCatalogFixture(t, `services:
  - id: broken
    number: 1
    name: Broken
    group: Test
    tvg_id: broken.example
    src: 1
    freq: 11494
    pol: x
    sr: 22000
    msys: cable
    pids: [100, 200, 100]
`)

	_, err := channels.LoadCatalogFile(path)
	if err == nil {
		t.Fatal("expected validation error")
	}
	for _, want := range []string{"services[0].pol", "services[0].msys", "services[0].pids", "PAT PID", "SDT PID"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("validation error %q missing %q", err.Error(), want)
		}
	}
}

func TestLoadCatalogFileRejectsUnknownFields(t *testing.T) {
	path := writeCatalogFixture(t, `services:
  - id: test-one-hd
    number: 101
    name: Test One HD
    group: Test
    tvg_id: test-one.example
    src: 1
    freq: 11494
    pol: h
    sr: 22000
    msys: dvbs2
    service_id: 1234
    pids: [0, 17, 7100, 7101, 7102]
`)

	_, err := channels.LoadCatalogFile(path)
	if err == nil {
		t.Fatal("expected unknown field error")
	}
	if !strings.Contains(err.Error(), "service_id") {
		t.Fatalf("validation error should mention invalid fields: %v", err)
	}
}

func TestLoadCatalogFileRejectsDuplicateIdentifiers(t *testing.T) {
	path := writeCatalogFixture(t, `services:
  - id: duplicate
    number: 1
    name: One
    group: Test
    tvg_id: one.example
    src: 1
    freq: 11494
    pol: h
    sr: 22000
    msys: dvbs2
    pids: [0, 17, 7100, 7101, 7102]
  - id: duplicate
    number: 1
    name: Two
    group: Test
    tvg_id: one.example
    src: 1
    freq: 11362
    pol: h
    sr: 22000
    msys: dvbs2
    pids: [0, 17, 7200, 7201, 7202]
`)

	_, err := channels.LoadCatalogFile(path)
	if err == nil {
		t.Fatal("expected duplicate validation error")
	}
	for _, want := range []string{"duplicate service id", "duplicate service number", "duplicate tvg_id"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error %q missing %q", err.Error(), want)
		}
	}
}

func writeCatalogFixture(t *testing.T, body string) string {
	t.Helper()
	file, err := os.CreateTemp(t.TempDir(), "catalog-*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.WriteString(body); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	return file.Name()
}
