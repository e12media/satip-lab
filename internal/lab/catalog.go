package lab

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/e12media/satip-lab/internal/channels"
)

type Catalog struct {
	Muxes    []Mux     `json:"muxes"`
	Services []Service `json:"services"`
}

type Mux struct {
	ID           string `json:"id"`
	Src          int    `json:"src"`
	Frequency    int    `json:"freq"`
	Polarization string `json:"pol"`
	SymbolRate   int    `json:"sr"`
	Delivery     string `json:"msys"`
}

type Service struct {
	ID        string `json:"id"`
	Number    int    `json:"number"`
	Name      string `json:"name"`
	Group     string `json:"group"`
	TvgID     string `json:"tvg_id"`
	MuxID     string `json:"mux_id"`
	ServiceID int    `json:"service_id"`
	PMTPID    int    `json:"pmt_pid"`
	VideoPID  int    `json:"video_pid"`
	AudioPID  int    `json:"audio_pid"`
}

func DefaultCatalog() Catalog {
	return CatalogFromChannels(channels.DachChannels)
}

func CatalogFromChannels(list []channels.Channel) Catalog {
	muxesByID := make(map[string]Mux)
	var muxes []Mux
	var services []Service

	for _, ch := range list {
		mux := Mux{
			ID:           muxID(ch.Src, ch.Frequency, ch.Polarization, ch.SymbolRate, ch.Delivery),
			Src:          ch.Src,
			Frequency:    ch.Frequency,
			Polarization: ch.Polarization,
			SymbolRate:   ch.SymbolRate,
			Delivery:     ch.Delivery,
		}
		if _, ok := muxesByID[mux.ID]; !ok {
			muxesByID[mux.ID] = mux
			muxes = append(muxes, mux)
		}
		services = append(services, Service{
			ID:        ch.ID,
			Number:    ch.Number,
			Name:      ch.Name,
			Group:     ch.Group,
			TvgID:     ch.TvgID,
			MuxID:     mux.ID,
			ServiceID: 1000 + ch.Number,
			PMTPID:    pidAt(ch.Pids, 2),
			VideoPID:  pidAt(ch.Pids, 3),
			AudioPID:  pidAt(ch.Pids, 4),
		})
	}

	return Catalog{Muxes: muxes, Services: services}
}

func (c Catalog) ServiceByID(id string) (Service, bool) {
	for _, svc := range c.Services {
		if svc.ID == id {
			return svc, true
		}
	}
	return Service{}, false
}

func (c Catalog) MuxByID(id string) (Mux, bool) {
	for _, mux := range c.Muxes {
		if mux.ID == id {
			return mux, true
		}
	}
	return Mux{}, false
}

func (c Catalog) MatchService(rawQuery string) (Service, Mux, error) {
	values, err := url.ParseQuery(rawQuery)
	if err != nil {
		return Service{}, Mux{}, ErrInvalidTune
	}
	src, freq, sr, err := parseTuneInts(values)
	if err != nil {
		return Service{}, Mux{}, ErrInvalidTune
	}
	pol := strings.ToLower(values.Get("pol"))
	msys := strings.ToLower(values.Get("msys"))

	for _, mux := range c.Muxes {
		if mux.Src == src &&
			mux.Frequency == freq &&
			mux.SymbolRate == sr &&
			strings.EqualFold(mux.Polarization, pol) &&
			strings.EqualFold(mux.Delivery, msys) {
			for _, svc := range c.Services {
				if svc.MuxID == mux.ID && serviceMatchesPIDs(svc, values.Get("pids")) {
					return svc, mux, nil
				}
			}
		}
	}
	return Service{}, Mux{}, ErrServiceNotFound
}

func (s Service) Channel(mux Mux) channels.Channel {
	return channels.Channel{
		ID:           s.ID,
		Number:       s.Number,
		Name:         s.Name,
		Group:        s.Group,
		TvgID:        s.TvgID,
		Frequency:    mux.Frequency,
		Polarization: mux.Polarization,
		SymbolRate:   mux.SymbolRate,
		Delivery:     mux.Delivery,
		Src:          mux.Src,
		Pids:         []int{0, 17, s.PMTPID, s.VideoPID, s.AudioPID},
	}
}

func (c Catalog) Channels() []channels.Channel {
	out := make([]channels.Channel, 0, len(c.Services))
	for _, svc := range c.Services {
		mux, ok := c.MuxByID(svc.MuxID)
		if !ok {
			continue
		}
		out = append(out, svc.Channel(mux))
	}
	return out
}

func muxID(src, freq int, pol string, sr int, msys string) string {
	return fmt.Sprintf("src%d-%d%s-%d-%s", src, freq, strings.ToLower(pol), sr, strings.ToLower(msys))
}

func pidAt(pids []int, idx int) int {
	if idx < len(pids) {
		return pids[idx]
	}
	return 0
}

func parseTuneInts(values url.Values) (int, int, int, error) {
	src, err := strconv.Atoi(values.Get("src"))
	if err != nil {
		return 0, 0, 0, err
	}
	freq, err := strconv.Atoi(values.Get("freq"))
	if err != nil {
		return 0, 0, 0, err
	}
	sr, err := strconv.Atoi(values.Get("sr"))
	if err != nil {
		return 0, 0, 0, err
	}
	return src, freq, sr, nil
}

func serviceMatchesPIDs(svc Service, raw string) bool {
	if raw == "" || strings.EqualFold(raw, "all") {
		return true
	}
	required := map[int]bool{
		svc.PMTPID:   false,
		svc.VideoPID: false,
		svc.AudioPID: false,
	}
	for _, part := range strings.Split(raw, ",") {
		pid, err := strconv.Atoi(strings.TrimSpace(part))
		if err != nil {
			continue
		}
		if _, ok := required[pid]; ok {
			required[pid] = true
		}
	}
	for _, seen := range required {
		if !seen {
			return false
		}
	}
	return true
}
