package epg

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"strings"
	"time"

	"github.com/e12media/satip-lab/internal/lab"
)

const (
	DefaultClock = "fixed:2026-03-29T01:30:00+01:00"
	timezone     = "Europe/Berlin"
)

type Clock struct {
	Mode     string    `json:"mode"`
	Now      time.Time `json:"now"`
	Timezone string    `json:"tz"`
}

type Options struct {
	Scenario lab.Scenario
}

type Metadata struct {
	LastModified time.Time
}

type tv struct {
	XMLName           xml.Name    `xml:"tv"`
	SourceInfoName    string      `xml:"source-info-name,attr"`
	GeneratorInfoName string      `xml:"generator-info-name,attr"`
	Date              string      `xml:"date,attr"`
	Channels          []channel   `xml:"channel"`
	Programmes        []programme `xml:"programme"`
}

type channel struct {
	ID          string      `xml:"id,attr"`
	DisplayName displayName `xml:"display-name"`
}

type displayName struct {
	Value string `xml:",chardata"`
}

type programme struct {
	Start      string     `xml:"start,attr"`
	Stop       string     `xml:"stop,attr"`
	Channel    string     `xml:"channel,attr"`
	Title      textValue  `xml:"title"`
	Desc       textValue  `xml:"desc"`
	EpisodeNum episodeNum `xml:"episode-num"`
}

type textValue struct {
	Lang  string `xml:"lang,attr,omitempty"`
	Value string `xml:",chardata"`
}

type episodeNum struct {
	System string `xml:"system,attr"`
	Value  string `xml:",chardata"`
}

func ParseClock(raw string) (Clock, error) {
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		return Clock{}, err
	}
	if strings.TrimSpace(raw) == "" {
		raw = DefaultClock
	}
	if strings.EqualFold(raw, "real") {
		return Clock{Mode: "real", Now: time.Now().In(loc), Timezone: timezone}, nil
	}
	if !strings.HasPrefix(raw, "fixed:") {
		return Clock{}, fmt.Errorf("invalid epg clock %q", raw)
	}
	parsed, err := time.Parse(time.RFC3339, strings.TrimPrefix(raw, "fixed:"))
	if err != nil {
		return Clock{}, fmt.Errorf("invalid fixed epg clock: %w", err)
	}
	return Clock{Mode: "fixed", Now: parsed.In(loc), Timezone: timezone}, nil
}

func GenerateXMLTV(catalog lab.Catalog, clock Clock, opts Options) ([]byte, Metadata, error) {
	metadata := Metadata{LastModified: clock.Now}
	if opts.Scenario.Name == lab.ScenarioEPGStale {
		metadata.LastModified = clock.Now.Add(-48 * time.Hour)
	}

	doc := tv{
		SourceInfoName:    "satip-lab",
		GeneratorInfoName: "satip-lab",
		Date:              formatXMLTVTime(clock.Now),
	}
	for _, service := range catalog.Services {
		id := xmltvChannelID(service, opts.Scenario)
		doc.Channels = append(doc.Channels, channel{
			ID:          id,
			DisplayName: displayName{Value: service.Name},
		})
		doc.Programmes = append(doc.Programmes, programmesForService(catalog, service, id, clock, opts.Scenario)...)
	}

	var b bytes.Buffer
	b.WriteString(xml.Header)
	enc := xml.NewEncoder(&b)
	enc.Indent("", "  ")
	if err := enc.Encode(doc); err != nil {
		return nil, Metadata{}, err
	}
	b.WriteByte('\n')
	return b.Bytes(), metadata, nil
}

func programmesForService(catalog lab.Catalog, service lab.Service, channelID string, clock Clock, scenario lab.Scenario) []programme {
	mux, _ := catalog.MuxByID(service.MuxID)
	step := density(service.ID)
	end := clock.Now.Add(24 * time.Hour)
	gapEnd := clock.Now.Add(time.Duration(gapDurationMinutes(scenario)) * time.Minute)
	var out []programme
	for start := clock.Now; start.Before(end); start = start.Add(step) {
		stop := start.Add(step)
		if stop.After(end) {
			stop = end
		}
		if scenario.Name == lab.ScenarioEPGGap && scenario.AppliesTo(service, mux) && !start.Before(clock.Now) && start.Before(gapEnd) {
			continue
		}
		out = append(out, programme{
			Start:   formatXMLTVTime(start),
			Stop:    formatXMLTVTime(stop),
			Channel: channelID,
			Title: textValue{
				Lang:  "en",
				Value: fmt.Sprintf("%s Lab Programme %02d:%02d", service.Name, start.In(clock.Now.Location()).Hour(), start.In(clock.Now.Location()).Minute()),
			},
			Desc: textValue{
				Lang:  "en",
				Value: fmt.Sprintf("Deterministic satip-lab EPG slot for %s.", service.Name),
			},
			EpisodeNum: episodeNum{
				System: "satip-lab",
				Value:  fmt.Sprintf("%s-%s", channelID, start.In(clock.Now.Location()).Format("20060102150405")),
			},
		})
	}
	return out
}

func xmltvChannelID(service lab.Service, scenario lab.Scenario) string {
	if scenario.Name == lab.ScenarioEPGMismatch && service.ID == "zdf-hd" {
		return "zdf-mismatch.invalid"
	}
	return service.TvgID
}

func density(serviceID string) time.Duration {
	switch serviceID {
	case "zdf-hd":
		return 30 * time.Minute
	case "arte-hd":
		return 45 * time.Minute
	case "phoenix-hd":
		return 90 * time.Minute
	case "3sat-hd":
		return 2 * time.Hour
	default:
		return time.Hour
	}
}

func gapDurationMinutes(scenario lab.Scenario) int {
	if scenario.DurationMin > 0 {
		return scenario.DurationMin
	}
	return 60
}

func formatXMLTVTime(t time.Time) string {
	return t.Format("20060102150405 -0700")
}
