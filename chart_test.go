package lyresdk

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/Lyrinox-Technologies/LyreSDK/wire"
)

func TestChartRequestValidationAndWithData(t *testing.T) {
	req := ChartRequest{
		ChartSpec: ChartSpec{Type: "bar", Title: "Top clients", Data: []ChartPoint{{Label: "alpha", Value: 1}}},
		Delivery:  ChartRefreshableSVG, RefreshSeconds: 60,
	}
	if err := req.Validate(); err != nil {
		t.Fatal(err)
	}
	next, err := req.WithData([]ChartPoint{{Label: "beta", Value: -2}})
	if err != nil {
		t.Fatal(err)
	}
	next.Data[0].Label = "changed"
	if req.Data[0].Label != "alpha" {
		t.Fatal("WithData mutated the original request")
	}
	for _, bad := range []ChartRequest{
		{ChartSpec: ChartSpec{Type: "pie", Title: "Bad", Data: []ChartPoint{}}},
		{ChartSpec: ChartSpec{Type: "bar", Title: "", Data: []ChartPoint{}}},
		{ChartSpec: ChartSpec{Type: "bar", Title: "Bad", Data: nil}},
		{ChartSpec: ChartSpec{Type: "line", Title: "Bad", Data: []ChartPoint{{Label: strings.Repeat("x", 129), Value: 1}}}},
		{ChartSpec: ChartSpec{Type: "bar", Title: "Bad", Data: []ChartPoint{{Label: "x", Value: 1e16}}}},
		{ChartSpec: ChartSpec{Type: "bar", Title: "Bad", Data: []ChartPoint{}}, Delivery: ChartDelivery("gif")},
	} {
		if err := bad.Validate(); err == nil {
			t.Fatalf("accepted invalid request: %#v", bad)
		}
	}
}

func TestCreateChartCallsStandardCapability(t *testing.T) {
	c, p := pair(t)
	c.mu.Lock()
	c.authenticated = true
	c.mu.Unlock()
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		var req wire.ClientToServicePayload
		receive(t, p, wire.MsgTypeClientToService, &req)
		if req.ToService != ChartCreateCapability {
			t.Errorf("reference = %q, want %q", req.ToService, ChartCreateCapability)
		}
		var input ChartRequest
		if err := json.Unmarshal(req.Payload, &input); err != nil {
			t.Error(err)
		}
		if input.Type != "line" || input.Delivery != ChartLiveSpec || len(input.Data) != 1 {
			t.Errorf("bad chart payload: %+v", input)
		}
		sendTest(t, p, wire.MsgTypeServiceResponse, &wire.ServiceResponsePayload{MessageID: req.MessageID, Success: true, Payload: []byte(`{"package_version":1,"delivery":"live_spec","spec":{"type":"line","title":"Events","description":"","data":[{"label":"today","value":3}],"width":800,"height":400},"svg":"<svg></svg>","update":{"strategy":"replace_data","refresh_seconds":60,"data_rule":"replace_all","patch_supported":false},"cache":{"key":"sha256:test","visibility":"private","max_age_seconds":0},"accessibility":{"title":"Events","description":"","table":[{"label":"today","value":3}]}}`)})
	}()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	pkg, err := c.CreateChart(ctx, ChartRequest{
		ChartSpec: ChartSpec{Type: "line", Title: "Events", Data: []ChartPoint{{Label: "today", Value: 3}}},
		Delivery:  ChartLiveSpec, RefreshSeconds: 60,
	})
	if err != nil {
		t.Fatal(err)
	}
	if pkg.PackageVersion != 1 || pkg.Update.Strategy != "replace_data" || pkg.Accessibility.Table[0].Value != 3 {
		t.Fatalf("bad package: %+v", pkg)
	}
	<-finished
}
