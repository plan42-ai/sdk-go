package eh

import (
	"encoding/json"
	"net/http"
)

// FeatureFlags holds feature flag overrides for a request.
type FeatureFlags struct {
	FeatureFlags map[string]bool `json:"-"`
}

// GetFeatureFlags returns the feature flag map.
func (f FeatureFlags) GetFeatureFlags() map[string]bool {
	return f.FeatureFlags
}

func processFeatureFlags(req *http.Request, flags FeatureFlags) {
	if len(flags.FeatureFlags) == 0 {
		return
	}
	b, err := json.Marshal(flags.FeatureFlags)
	if err != nil {
		return
	}
	req.Header.Set("X-EventHorizon-FeatureFlags", string(b))
}
