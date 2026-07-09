package ts

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const chunkSize = 1316
const DefaultSampleAssetDir = "assets"

const (
	SampleProfileSynthetic    = "synthetic"
	SampleProfileH264AACShort = "h264_aac_short"
	SampleProfileH264Silent   = "h264_silent"
)

type Source struct {
	Path           string
	MediaDir       string
	SampleProfile  string
	SampleAssetDir string
}

func (s *Source) LoadPayload() ([]byte, error) {
	data, err := os.ReadFile(s.Path)
	if err == nil && len(data) > 0 {
		return data, nil
	}
	return syntheticNullTransport(), nil
}

func (s *Source) LoadServicePayload(profile ServiceProfile) ([]byte, error) {
	return s.LoadServicePayloadWithOptions(profile, EITOptions{Now: defaultEITNow()})
}

func (s *Source) LoadServicePayloadWithOptions(profile ServiceProfile, eit EITOptions) ([]byte, error) {
	data, err := os.ReadFile(s.Path)
	if err == nil && len(data) > 0 {
		return data, nil
	}
	if media, ok, err := s.loadMediaDirPayload(profile); err != nil {
		return nil, err
	} else if ok {
		return media, nil
	}
	if sampleName, ok := s.sampleAssetName(profile); ok {
		return os.ReadFile(filepath.Join(s.sampleAssetDir(), sampleName))
	}
	return SyntheticServiceTransportWithOptions(profile, eit), nil
}

func (s *Source) loadMediaDirPayload(profile ServiceProfile) ([]byte, bool, error) {
	if strings.TrimSpace(s.MediaDir) == "" {
		return nil, false, nil
	}
	filename := serviceMediaFilename(profile.ID)
	if filename == "" {
		return nil, false, nil
	}
	path := filepath.Join(s.MediaDir, filename)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, false, nil
		}
		return nil, false, err
	}
	if len(data) == 0 {
		return nil, false, fmt.Errorf("media asset %q is empty", path)
	}
	return data, true, nil
}

func serviceMediaFilename(serviceID string) string {
	id := strings.TrimSpace(serviceID)
	if id == "" {
		return ""
	}
	return filepath.Base(id) + ".ts"
}

func (s *Source) sampleAssetName(profile ServiceProfile) (string, bool) {
	if profile.ID != "zdf-hd" {
		return "", false
	}
	switch s.SampleProfile {
	case SampleProfileH264AACShort:
		return "h264_aac_short.ts", true
	case SampleProfileH264Silent:
		return "h264_silent.ts", true
	default:
		return "", false
	}
}

func (s *Source) sampleAssetDir() string {
	if s.SampleAssetDir != "" {
		return s.SampleAssetDir
	}
	return DefaultSampleAssetDir
}

func syntheticNullTransport() []byte {
	packet := make([]byte, 188)
	packet[0] = 0x47
	packet[1] = 0x1F
	packet[2] = 0xFF
	packet[3] = 0x10
	out := make([]byte, 0, 188*800)
	for i := 0; i < 800; i++ {
		out = append(out, packet...)
	}
	return out
}

func (s *Source) ChunkAt(payload []byte, offset int) ([]byte, int) {
	if len(payload) == 0 {
		return nil, 0
	}
	if offset < 0 || offset >= len(payload) {
		offset = 0
	}
	end := offset + chunkSize
	if end >= len(payload) {
		chunk := payload[offset:]
		return chunk, 0
	}
	return payload[offset:end], end
}
