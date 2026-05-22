package channels

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

type catalogFile struct {
	Services []channelFile `yaml:"services"`
}

type channelFile struct {
	ID           string `yaml:"id"`
	Number       int    `yaml:"number"`
	Name         string `yaml:"name"`
	Group        string `yaml:"group"`
	TvgID        string `yaml:"tvg_id"`
	Frequency    int    `yaml:"freq"`
	Polarization string `yaml:"pol"`
	SymbolRate   int    `yaml:"sr"`
	Delivery     string `yaml:"msys"`
	Src          int    `yaml:"src"`
	Pids         []int  `yaml:"pids"`
}

func LoadCatalogFile(path string) ([]Channel, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read catalog %q: %w", path, err)
	}
	var file catalogFile
	decoder := yaml.NewDecoder(bytes.NewReader(body))
	decoder.KnownFields(true)
	if err := decoder.Decode(&file); err != nil {
		return nil, fmt.Errorf("parse catalog %q: %w", path, err)
	}
	channels, err := file.channels()
	if err != nil {
		return nil, fmt.Errorf("catalog %q: %w", path, err)
	}
	return channels, nil
}

func (f catalogFile) channels() ([]Channel, error) {
	if len(f.Services) == 0 {
		return nil, errors.New("services must contain at least one entry")
	}
	out := make([]Channel, 0, len(f.Services))
	ids := make(map[string]int)
	numbers := make(map[int]int)
	tvgIDs := make(map[string]int)
	var problems []string
	for i, svc := range f.Services {
		prefix := fmt.Sprintf("services[%d]", i)
		ch := Channel{
			ID:           strings.TrimSpace(svc.ID),
			Number:       svc.Number,
			Name:         strings.TrimSpace(svc.Name),
			Group:        strings.TrimSpace(svc.Group),
			TvgID:        strings.TrimSpace(svc.TvgID),
			Frequency:    svc.Frequency,
			Polarization: strings.ToLower(strings.TrimSpace(svc.Polarization)),
			SymbolRate:   svc.SymbolRate,
			Delivery:     strings.ToLower(strings.TrimSpace(svc.Delivery)),
			Src:          svc.Src,
			Pids:         append([]int(nil), svc.Pids...),
		}
		problems = append(problems, validateChannel(prefix, ch)...)
		if prev, ok := ids[ch.ID]; ok && ch.ID != "" {
			problems = append(problems, fmt.Sprintf("%s.id duplicate service id %q also used by services[%d]", prefix, ch.ID, prev))
		}
		if prev, ok := numbers[ch.Number]; ok && ch.Number > 0 {
			problems = append(problems, fmt.Sprintf("%s.number duplicate service number %d also used by services[%d]", prefix, ch.Number, prev))
		}
		if prev, ok := tvgIDs[ch.TvgID]; ok && ch.TvgID != "" {
			problems = append(problems, fmt.Sprintf("%s.tvg_id duplicate tvg_id %q also used by services[%d]", prefix, ch.TvgID, prev))
		}
		ids[ch.ID] = i
		numbers[ch.Number] = i
		tvgIDs[ch.TvgID] = i
		out = append(out, ch)
	}
	if len(problems) > 0 {
		return nil, errors.New(strings.Join(problems, "; "))
	}
	return out, nil
}

func validateChannel(prefix string, ch Channel) []string {
	var problems []string
	if ch.ID == "" {
		problems = append(problems, prefix+".id is required")
	}
	if ch.Number <= 0 {
		problems = append(problems, prefix+".number must be > 0")
	}
	if ch.Name == "" {
		problems = append(problems, prefix+".name is required")
	}
	if ch.Group == "" {
		problems = append(problems, prefix+".group is required")
	}
	if ch.TvgID == "" {
		problems = append(problems, prefix+".tvg_id is required")
	}
	if ch.Src <= 0 {
		problems = append(problems, prefix+".src must be > 0")
	}
	if ch.Frequency <= 0 {
		problems = append(problems, prefix+".freq must be > 0")
	}
	if ch.Polarization != "h" && ch.Polarization != "v" {
		problems = append(problems, prefix+".pol must be h or v")
	}
	if ch.SymbolRate <= 0 {
		problems = append(problems, prefix+".sr must be > 0")
	}
	if ch.Delivery != "dvbs" && ch.Delivery != "dvbs2" {
		problems = append(problems, prefix+".msys must be dvbs or dvbs2")
	}
	if len(ch.Pids) < 5 {
		problems = append(problems, prefix+".pids must contain PAT, SDT, PMT, video, and audio PIDs")
	}
	if len(ch.Pids) > 0 && ch.Pids[0] != 0 {
		problems = append(problems, prefix+".pids[0] PAT PID must be 0")
	}
	if len(ch.Pids) > 1 && ch.Pids[1] != 17 {
		problems = append(problems, prefix+".pids[1] SDT PID must be 17")
	}
	for idx, pid := range ch.Pids {
		if pid < 0 || pid > 8191 {
			problems = append(problems, fmt.Sprintf("%s.pids[%d] must be between 0 and 8191", prefix, idx))
		}
	}
	return problems
}
