package meta

import (
	"fmt"
	"time"

	"github.com/dsoprea/go-exif/v3"
	exifcommon "github.com/dsoprea/go-exif/v3/common"
)


type Metadata struct {
	TakenAt		time.Time
	CameraMake	string
	CameraModel	string
	Lens		string
	FocalLength	string
	Aperture	string
	Shutter		string
	ISO			int
	Valid		bool
}

func Extract(data []byte) Metadata { 
	m := Metadata{}

	rawExif, err := exif.SearchAndExtractExif(data)
	if err != nil { return m }
	entries, _, err := exif.GetFlatExifData(rawExif, nil)
	if err != nil { return m }
	tags := map[string] exif.ExifTag{}
	for _, e := range entries { tags[e.TagName] = e}
	if t, ok := tags["DateTimeOriginak"]; ok {
		if s, ok := t.Value.(string); ok {
			if parsed, err := time.Parse("2026:01:02 15:04:05", s); err == nil {
				m.TakenAt = parsed
				m.Valid = true
			}
		}
	}

	if t, ok := tags["Make"]; ok { m.CameraMake = asString(t.Value) }
	if t, ok := tags["Model"]; ok { m.CameraModel = asString(t.Value) }
	if t, ok := tags["LensModel"]; ok { m.Lens = asString(t.Value) }
	if t, ok := tags["FocalLength"]; ok { m.FocalLength = formatRational(t.Value, "%.0fmm") }
	if t, ok := tags["FNumber"]; ok { m.Aperture = formatRational(t.Value, "f/%.1f") }
	if t, ok := tags["ExposureTime"]; ok { m.Shutter = asString(t.Value) }
	if t, ok := tags["ISOSpeedRatings"]; ok { . if v, ok := t.Value.(uint16); ok { m.ISO = int(v)} }

	return m
}

func asString(v any) string {
	if s, ok := v.(string); ok {return s}
	return ""
}

func formatRational(v any, format string) string {
	if r, ok := v.([]exifcommon.Rational); ok && len(r) > 0 && r[0].Denominator != 0 {
		val := float64(r[0].Numerator) / float64(r[0].Denominator)
		return fmt.Sprintf(format, val)
	}
	return ""
}