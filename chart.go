package lyresdk

import (
	"context"
	"fmt"
	"math"
	"unicode/utf8"
)

const ChartCreateCapability = "lyre.chart.create@v1"

type ChartDelivery string

const (
	ChartStaticSVG      ChartDelivery = "static_svg"
	ChartRefreshableSVG ChartDelivery = "refreshable_svg"
	ChartLiveSpec       ChartDelivery = "live_spec"
)

type ChartPoint struct {
	Label string  `json:"label"`
	Value float64 `json:"value"`
}
type ChartSpec struct {
	Type        string       `json:"type"`
	Title       string       `json:"title"`
	Description string       `json:"description,omitempty"`
	Data        []ChartPoint `json:"data"`
	Width       int          `json:"width,omitempty"`
	Height      int          `json:"height,omitempty"`
}
type ChartRequest struct {
	ChartSpec
	Delivery       ChartDelivery `json:"delivery,omitempty"`
	RefreshSeconds int           `json:"refresh_seconds,omitempty"`
}
type ChartPackage struct {
	PackageVersion int                `json:"package_version"`
	Delivery       ChartDelivery      `json:"delivery"`
	Spec           ChartSpec          `json:"spec"`
	SVG            string             `json:"svg"`
	Update         ChartUpdate        `json:"update"`
	Cache          ChartCache         `json:"cache"`
	Accessibility  ChartAccessibility `json:"accessibility"`
}
type ChartUpdate struct {
	Strategy       string `json:"strategy"`
	RefreshSeconds int    `json:"refresh_seconds"`
	DataRule       string `json:"data_rule"`
	PatchSupported bool   `json:"patch_supported"`
}
type ChartCache struct {
	Key           string `json:"key"`
	Visibility    string `json:"visibility"`
	MaxAgeSeconds int    `json:"max_age_seconds"`
}
type ChartAccessibility struct {
	Title       string       `json:"title"`
	Description string       `json:"description"`
	Table       []ChartPoint `json:"table"`
}

// Validate checks v1 bounds locally. The capability remains authoritative.
func (r ChartRequest) Validate() error {
	if r.Type != "bar" && r.Type != "line" {
		return fmt.Errorf("chart type must be bar or line")
	}
	if !utf8.ValidString(r.Title) || utf8.RuneCountInString(r.Title) < 1 || utf8.RuneCountInString(r.Title) > 256 {
		return fmt.Errorf("chart title must contain 1-256 characters")
	}
	if !utf8.ValidString(r.Description) || utf8.RuneCountInString(r.Description) > 2048 {
		return fmt.Errorf("chart description exceeds 2048 characters")
	}
	if r.Width != 0 && (r.Width < 320 || r.Width > 1600) || r.Height != 0 && (r.Height < 240 || r.Height > 1200) {
		return fmt.Errorf("chart dimensions exceed v1 bounds")
	}
	switch r.Delivery {
	case "", ChartStaticSVG, ChartRefreshableSVG, ChartLiveSpec:
	default:
		return fmt.Errorf("invalid chart delivery mode")
	}
	if r.RefreshSeconds < 0 || r.RefreshSeconds > 86400 {
		return fmt.Errorf("refresh seconds must be 1-86400 or zero for default")
	}
	if r.Data == nil || len(r.Data) > 200 {
		return fmt.Errorf("chart data must be a non-nil slice with at most 200 points")
	}
	for _, p := range r.Data {
		if !utf8.ValidString(p.Label) || utf8.RuneCountInString(p.Label) > 128 || math.IsNaN(p.Value) || math.IsInf(p.Value, 0) || math.Abs(p.Value) > 1e15 {
			return fmt.Errorf("invalid chart point")
		}
	}
	return nil
}

// WithData prepares a full replacement without mutating the original request.
// Reinvoke CreateChart to get a fresh SVG, accessibility table, and cache key.
func (r ChartRequest) WithData(points []ChartPoint) (ChartRequest, error) {
	r.Data = append([]ChartPoint{}, points...)
	return r, r.Validate()
}

func (c *Client) CreateChart(ctx context.Context, request ChartRequest) (*ChartPackage, error) {
	if err := request.Validate(); err != nil {
		return nil, err
	}
	var result ChartPackage
	if err := c.Call(ctx, ChartCreateCapability, request, &result); err != nil {
		return nil, err
	}
	return &result, nil
}
func (s *Service) CreateChart(ctx context.Context, request ChartRequest) (*ChartPackage, error) {
	c, err := s.connection()
	if err != nil {
		return nil, err
	}
	return c.CreateChart(ctx, request)
}
