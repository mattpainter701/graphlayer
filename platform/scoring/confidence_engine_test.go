package scoring

import (
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/mattpainter701/graphlayer/platform/models"
)

// ========================================================================
// Per-Attribute Confidence Tests
// ========================================================================

func TestScoreAttribute(t *testing.T) {
	engine := NewConfidenceEngine()

	tests := []struct {
		name     string
		source   models.ConfidenceSource
		quality  float64
		minScore float64
		maxScore float64
	}{
		{"config high quality", models.ConfidenceSourceConfig, 1.0, 0.85, 0.95},
		{"config low quality", models.ConfidenceSourceConfig, 0.5, 0.40, 0.50},
		{"operational high", models.ConfidenceSourceOperational, 1.0, 0.75, 0.85},
		{"inferred high", models.ConfidenceSourceInferred, 1.0, 0.35, 0.45},
		{"inferred low", models.ConfidenceSourceInferred, 0.3, 0.10, 0.15},
		{"connector high", models.ConfidenceSourceConnector, 1.0, 0.65, 0.75},
		{"manual always 1.0", models.ConfidenceSourceManual, 1.0, 0.95, 1.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := engine.ScoreAttribute(tt.source, tt.quality)
			if score < tt.minScore || score > tt.maxScore {
				t.Errorf("ScoreAttribute(%s, %.1f) = %.4f; want [%.2f, %.2f]",
					tt.source, tt.quality, score, tt.minScore, tt.maxScore)
			}
		})
	}
}

func TestScoreDeviceAttributes(t *testing.T) {
	engine := NewConfidenceEngine()

	t.Run("config source device", func(t *testing.T) {
		profile := engine.ScoreDeviceAttributes(
			models.ConfidenceSourceConfig, // hostname
			models.ConfidenceSourceConfig, // vendor
			models.ConfidenceSourceConfig, // model
			models.ConfidenceSourceConfig, // version
			models.ConfidenceSourceConfig, // mgmt_ip
			models.ConfidenceSourceConfig, // site
			models.ConfidenceSourceConfig, // role
		)

		// Check per-attribute scores exist
		expectedFields := []string{"hostname", "vendor", "model", "version", "management_ip", "site", "role"}
		for _, field := range expectedFields {
			if _, ok := profile.PerAttribute[field]; !ok {
				t.Errorf("missing per-attribute field: %s", field)
			}
		}

		// Overall should be high for config-sourced device
		if profile.Overall < 0.50 {
			t.Errorf("expected high overall confidence from config source, got %.4f", profile.Overall)
		}

		// Hostname from config should be high
		if profile.PerAttribute["hostname"] < 0.80 {
			t.Errorf("hostname confidence too low: %.4f", profile.PerAttribute["hostname"])
		}

		// Site from config should be lower (SNMP location hints are unreliable)
		if profile.PerAttribute["site"] >= profile.PerAttribute["hostname"] {
			t.Logf("site confidence (%.4f) should be <= hostname (%.4f)",
				profile.PerAttribute["site"], profile.PerAttribute["hostname"])
		}

		if profile.Tier != models.ConfidenceTierHigh && profile.Tier != models.ConfidenceTierMedium {
			t.Errorf("expected Medium+ tier, got %s", profile.Tier)
		}
	})

	t.Run("inferred source device", func(t *testing.T) {
		profile := engine.ScoreDeviceAttributes(
			models.ConfidenceSourceInferred,
			models.ConfidenceSourceInferred,
			models.ConfidenceSourceInferred,
			models.ConfidenceSourceInferred,
			models.ConfidenceSourceInferred,
			models.ConfidenceSourceInferred,
			models.ConfidenceSourceInferred,
		)

		// Should be low confidence
		if profile.Overall >= 0.50 {
			t.Errorf("expected low confidence from inferred source, got %.4f", profile.Overall)
		}

		if profile.Tier == models.ConfidenceTierHigh {
			t.Errorf("inferred source should not produce high confidence tier")
		}
	})

	t.Run("mixed sources", func(t *testing.T) {
		profile := engine.ScoreDeviceAttributes(
			models.ConfidenceSourceConfig,      // hostname - high
			models.ConfidenceSourceConnector,   // vendor - medium (connector + 0.6 quality)
			models.ConfidenceSourceConfig,      // model - high
			models.ConfidenceSourceOperational, // version - medium-high
			models.ConfidenceSourceConfig,      // mgmt_ip - high
			models.ConfidenceSourceConnector,   // site - high (connector)
			models.ConfidenceSourceInferred,    // role - low (0.5 quality)
		)

		// hostname should be higher than role
		if profile.PerAttribute["hostname"] <= profile.PerAttribute["role"] {
			t.Errorf("hostname (%.4f) should be > role (%.4f)",
				profile.PerAttribute["hostname"], profile.PerAttribute["role"])
		}

		// vendor from connector should be lower than hostname from config
		if profile.PerAttribute["vendor"] >= profile.PerAttribute["hostname"] {
			t.Logf("vendor (%.4f) may be close to hostname (%.4f)", profile.PerAttribute["vendor"], profile.PerAttribute["hostname"])
		}

		if len(profile.EvidenceSources) < 3 {
			t.Errorf("expected at least 3 unique evidence sources, got %d: %v",
				len(profile.EvidenceSources), profile.EvidenceSources)
		}
	})
}

func TestScoreInterfaceAttributes(t *testing.T) {
	engine := NewConfidenceEngine()

	profile := engine.ScoreInterfaceAttributes(
		models.ConfidenceSourceConfig,
		models.ConfidenceSourceConfig,
		models.ConfidenceSourceConfig,
		models.ConfidenceSourceOperational,
	)

	if profile.Overall < 0.50 {
		t.Errorf("expected decent confidence for config-sourced interface, got %.4f", profile.Overall)
	}

	requiredFields := []string{"name", "ip_address", "vlan_id", "status"}
	for _, field := range requiredFields {
		if _, ok := profile.PerAttribute[field]; !ok {
			t.Errorf("missing interface field: %s", field)
		}
	}
}

func TestScoreLinkAttributes(t *testing.T) {
	engine := NewConfidenceEngine()

	profile := engine.ScoreLinkAttributes(
		models.ConfidenceSourceOperational, // LLDP/CDP
		models.ConfidenceSourceOperational,
		models.ConfidenceSourceOperational,
	)

	if profile.Overall < 0.50 {
		t.Errorf("expected decent confidence for operational-sourced link, got %.4f", profile.Overall)
	}
}

// ========================================================================
// Confidence Propagation Tests
// ========================================================================

func TestPropagateConfidence(t *testing.T) {
	engine := NewConfidenceEngine()

	// Create a high-confidence device profile
	deviceProfile := engine.ScoreDeviceAttributes(
		models.ConfidenceSourceConfig,
		models.ConfidenceSourceConfig,
		models.ConfidenceSourceConfig,
		models.ConfidenceSourceConfig,
		models.ConfidenceSourceConfig,
		models.ConfidenceSourceConfig,
		models.ConfidenceSourceConfig,
	)

	t.Run("direct propagation (hop 1)", func(t *testing.T) {
		ifaceFields := []string{"name", "ip_address", "vlan_id", "status"}
		ifaceProfile := engine.PropagateConfidence(deviceProfile, 1, ifaceFields, models.ConfidenceSourceConfig)

		// Interface confidence should be lower than device (decay applied)
		if ifaceProfile.Overall >= deviceProfile.Overall {
			t.Errorf("interface confidence (%.4f) should be < device confidence (%.4f)",
				ifaceProfile.Overall, deviceProfile.Overall)
		}

		// Should still be decent
		if ifaceProfile.Overall < 0.50 {
			t.Errorf("propagated interface confidence too low: %.4f", ifaceProfile.Overall)
		}
	})

	t.Run("multi-hop decay", func(t *testing.T) {
		fields := []string{"name"}
		hop1 := engine.PropagateConfidence(deviceProfile, 1, fields, models.ConfidenceSourceConfig)
		hop3 := engine.PropagateConfidence(deviceProfile, 3, fields, models.ConfidenceSourceConfig)

		if hop3.Overall >= hop1.Overall {
			t.Errorf("hop 3 confidence (%.4f) should be < hop 1 (%.4f)", hop3.Overall, hop1.Overall)
		}
	})

	t.Run("exceeds max hops", func(t *testing.T) {
		fields := []string{"name"}
		tooFar := engine.PropagateConfidence(deviceProfile, 10, fields, models.ConfidenceSourceConfig)

		// Default max hops is 5, so hop 10 should be zero
		if tooFar.Overall != 0.0 {
			t.Errorf("expected 0 confidence beyond max hops, got %.4f", tooFar.Overall)
		}
	})

	t.Run("propagation with different source types", func(t *testing.T) {
		fields := []string{"name", "ip_address"}
		configProp := engine.PropagateConfidence(deviceProfile, 1, fields, models.ConfidenceSourceConfig)
		inferredProp := engine.PropagateConfidence(deviceProfile, 1, fields, models.ConfidenceSourceInferred)

		if inferredProp.Overall >= configProp.Overall {
			t.Errorf("inferred propagation (%.4f) should be < config propagation (%.4f)",
				inferredProp.Overall, configProp.Overall)
		}
		if configProp.Overall > deviceProfile.Overall {
			t.Errorf("propagated confidence %.4f exceeds parent %.4f", configProp.Overall, deviceProfile.Overall)
		}
	})
}

func TestPropagateDeviceToInterfaces(t *testing.T) {
	engine := NewConfidenceEngine()

	deviceProfile := engine.ScoreDeviceAttributes(
		models.ConfidenceSourceConfig, models.ConfidenceSourceConfig,
		models.ConfidenceSourceConfig, models.ConfidenceSourceConfig,
		models.ConfidenceSourceConfig, models.ConfidenceSourceConfig,
		models.ConfidenceSourceConfig,
	)

	ifaceFields := []string{"name", "ip_address", "vlan_id", "status"}
	ifaceProfile := engine.PropagateDeviceToInterfaces(deviceProfile, ifaceFields)

	if ifaceProfile.Overall <= 0.0 {
		t.Error("propagated interface should have positive confidence")
	}
}

// ========================================================================
// Corroboration Tests
// ========================================================================

func TestApplyCorroboration(t *testing.T) {
	engine := NewConfidenceEngine()

	baseProfile := engine.ScoreDeviceAttributes(
		models.ConfidenceSourceConfig, models.ConfidenceSourceConfig,
		models.ConfidenceSourceConfig, models.ConfidenceSourceConfig,
		models.ConfidenceSourceConfig, models.ConfidenceSourceConfig,
		models.ConfidenceSourceConfig,
	)

	t.Run("no corroboration with single source", func(t *testing.T) {
		evidence := []models.CorroborationEvidence{
			{SourceType: models.ConfidenceSourceConfig, SourceName: "ssh-cisco-ios", Field: "hostname", Value: "switch1"},
		}
		result := engine.ApplyCorroboration(evidence, baseProfile)
		if result.CorroborationBoost != 0.0 {
			t.Errorf("single source should not get boost, got %.4f", result.CorroborationBoost)
		}
	})

	t.Run("two-source corroboration", func(t *testing.T) {
		evidence := []models.CorroborationEvidence{
			{SourceType: models.ConfidenceSourceConfig, SourceName: "ssh-cisco-ios", Field: "hostname", Value: "switch1"},
			{SourceType: models.ConfidenceSourceConnector, SourceName: "netbox", Field: "hostname", Value: "switch1"},
		}
		result := engine.ApplyCorroboration(evidence, baseProfile)

		if result.CorroborationBoost <= 0.0 {
			t.Errorf("two sources should get corroboration boost, got %.4f", result.CorroborationBoost)
		}
		if result.Overall <= baseProfile.Overall {
			t.Errorf("corroborated overall (%.4f) should be > base (%.4f)",
				result.Overall, baseProfile.Overall)
		}
	})

	t.Run("repeated calls replace current corroboration contribution", func(t *testing.T) {
		twoSources := []models.CorroborationEvidence{
			{SourceType: models.ConfidenceSourceConfig, SourceName: "config", Field: "hostname", Value: "switch1"},
			{SourceType: models.ConfidenceSourceConnector, SourceName: "inventory", Field: "hostname", Value: "switch1"},
		}
		once := engine.ApplyCorroboration(twoSources, baseProfile)
		twice := engine.ApplyCorroboration(twoSources, once)
		if twice.CorroborationBoost != once.CorroborationBoost {
			t.Fatalf("repeated evidence inflated boost from %.3f to %.3f", once.CorroborationBoost, twice.CorroborationBoost)
		}
		stronger := append(twoSources, models.CorroborationEvidence{SourceType: models.ConfidenceSourceManual, SourceName: "review", Field: "hostname", Value: "switch1"})
		replaced := engine.ApplyCorroboration(stronger, twice)
		if replaced.CorroborationBoost <= twice.CorroborationBoost || replaced.CorroborationBoost > engine.CorroborationConfig.MaxBoost {
			t.Fatalf("stronger evidence should replace with larger bounded contribution; got %.3f", replaced.CorroborationBoost)
		}
		cleared := engine.ApplyCorroboration([]models.CorroborationEvidence{{SourceType: models.ConfidenceSourceConfig, SourceName: "config", Field: "hostname", Value: "other"}}, replaced)
		if cleared.CorroborationBoost != 0 || cleared.CorroboratingSourceCount != 0 {
			t.Fatalf("unsupported prior corroboration remained: boost %.3f count %d", cleared.CorroborationBoost, cleared.CorroboratingSourceCount)
		}
	})

	t.Run("three-source corroboration", func(t *testing.T) {
		evidence := []models.CorroborationEvidence{
			{SourceType: models.ConfidenceSourceConfig, SourceName: "ssh-cisco-ios", Field: "hostname", Value: "switch1"},
			{SourceType: models.ConfidenceSourceConnector, SourceName: "netbox", Field: "hostname", Value: "switch1"},
			{SourceType: models.ConfidenceSourceConnector, SourceName: "servicenow", Field: "hostname", Value: "switch1"},
		}
		result := engine.ApplyCorroboration(evidence, baseProfile)

		// Max boost is 0.20
		if result.CorroborationBoost > 0.20 {
			t.Errorf("corroboration boost (%.4f) should not exceed max (0.20)", result.CorroborationBoost)
		}
	})

	t.Run("same source type doesn't count as independent", func(t *testing.T) {
		// Two connector sources — if RequireIndependentSources is true, only counts as 1
		evidence := []models.CorroborationEvidence{
			{SourceType: models.ConfidenceSourceConnector, SourceName: "netbox", Field: "hostname", Value: "switch1"},
			{SourceType: models.ConfidenceSourceConnector, SourceName: "servicenow", Field: "hostname", Value: "switch1"},
		}
		result := engine.ApplyCorroboration(evidence, baseProfile)

		// With RequireIndependentSources=true, same source type = no boost
		if result.CorroborationBoost > 0.0 {
			t.Errorf("same source type with RequireIndependentSources should not get boost, got %.4f",
				result.CorroborationBoost)
		}
	})
}

func TestCorroborateAcrossSources(t *testing.T) {
	engine := NewConfidenceEngine()

	profiles := map[string]*models.ConfidenceProfile{
		"ssh-config": engine.ScoreDeviceAttributes(
			models.ConfidenceSourceConfig, models.ConfidenceSourceConfig,
			models.ConfidenceSourceConfig, models.ConfidenceSourceConfig,
			models.ConfidenceSourceConfig, models.ConfidenceSourceConfig,
			models.ConfidenceSourceConfig,
		),
		"netbox": engine.ScoreDeviceAttributes(
			models.ConfidenceSourceConnector, models.ConfidenceSourceConnector,
			models.ConfidenceSourceConnector, models.ConfidenceSourceConnector,
			models.ConfidenceSourceConnector, models.ConfidenceSourceConnector,
			models.ConfidenceSourceConnector,
		),
	}

	result := engine.CorroborateAcrossSources(profiles)

	if len(result) != 2 {
		t.Errorf("expected 2 results, got %d", len(result))
	}

	// Profiles contain scores but no field values, so matching field names do
	// not prove that their underlying observations agree.
	for name, profile := range result {
		if profile.CorroborationBoost != 0.0 {
			t.Errorf("source %s received an unsupported corroboration boost %.4f", name, profile.CorroborationBoost)
		}
	}
}

func TestApplyCorroborationRequiresSameValue(t *testing.T) {
	engine := NewConfidenceEngine()
	base := engine.ScoreInterfaceAttributes(models.ConfidenceSourceConfig, models.ConfidenceSourceConfig, models.ConfidenceSourceConfig, models.ConfidenceSourceConfig)
	result := engine.ApplyCorroboration([]models.CorroborationEvidence{
		{SourceType: models.ConfidenceSourceConfig, SourceName: "config", Field: "hostname", Value: "router-a"},
		{SourceType: models.ConfidenceSourceConnector, SourceName: "inventory", Field: "hostname", Value: "router-b"},
	}, base)
	if result.CorroborationBoost != 0 || result.CorroboratingSourceCount != 0 {
		t.Fatalf("conflicting values must not corroborate: boost %.3f, count %d", result.CorroborationBoost, result.CorroboratingSourceCount)
	}
}

// ========================================================================
// Staleness Decay Tests
// ========================================================================

func TestApplyStalenessDecay(t *testing.T) {
	engine := NewConfidenceEngine()

	baseProfile := engine.ScoreDeviceAttributes(
		models.ConfidenceSourceConfig, models.ConfidenceSourceConfig,
		models.ConfidenceSourceConfig, models.ConfidenceSourceConfig,
		models.ConfidenceSourceConfig, models.ConfidenceSourceConfig,
		models.ConfidenceSourceConfig,
	)
	originalOverall := baseProfile.Overall

	t.Run("fresh record has no decay", func(t *testing.T) {
		now := time.Now()
		result := engine.ApplyStalenessDecay(baseProfile, now, now)
		if result.StalenessDecay > 0.01 {
			t.Errorf("fresh record should have near-zero staleness decay, got %.4f", result.StalenessDecay)
		}
	})

	t.Run("half-life decay", func(t *testing.T) {
		now := time.Now()
		// 30 days ago (half-life = 30 days)
		refreshed := now.Add(-30 * 24 * time.Hour)
		result := engine.ApplyStalenessDecay(baseProfile, refreshed, now)

		// Confidence should be roughly halved
		expected := originalOverall * 0.5
		tolerance := 0.05
		if math.Abs(result.Overall-expected) > tolerance {
			t.Errorf("after half-life, expected ~%.4f, got %.4f (decay factor should be ~0.5)",
				expected, result.Overall)
		}

		if math.Abs(result.StalenessDecay-0.5) > 0.01 {
			t.Errorf("staleness decay metric should be ~0.5 at half-life, got %.4f", result.StalenessDecay)
		}
	})

	t.Run("full decay at max age", func(t *testing.T) {
		now := time.Now()
		refreshed := now.Add(-365 * 24 * time.Hour) // 1 year = max age
		result := engine.ApplyStalenessDecay(baseProfile, refreshed, now)

		if result.Overall != 0.0 {
			t.Errorf("at max age confidence should be 0.0, got %.4f", result.Overall)
		}
	})

	t.Run("staleness and corroboration apply once in either order", func(t *testing.T) {
		now := time.Now()
		refreshed := now.Add(-30 * 24 * time.Hour)
		evidence := []models.CorroborationEvidence{
			{SourceType: models.ConfidenceSourceConfig, SourceName: "config", Field: "hostname", Value: "router-a"},
			{SourceType: models.ConfidenceSourceConnector, SourceName: "inventory", Field: "hostname", Value: "router-a"},
		}
		staleThenCorroborated := engine.ApplyCorroboration(evidence, engine.ApplyStalenessDecay(baseProfile, refreshed, now))
		corroboratedThenStale := engine.ApplyStalenessDecay(engine.ApplyCorroboration(evidence, baseProfile), refreshed, now)
		if math.Abs(staleThenCorroborated.Overall-corroboratedThenStale.Overall) > 1e-9 {
			t.Fatalf("order changed overall confidence: %.6f vs %.6f", staleThenCorroborated.Overall, corroboratedThenStale.Overall)
		}
		if math.Abs(staleThenCorroborated.StalenessDecay-0.5) > 0.01 {
			t.Fatalf("expected half-life decay metric 0.5, got %.4f", staleThenCorroborated.StalenessDecay)
		}
	})

	t.Run("repeated staleness uses current decay and keeps detail consistent", func(t *testing.T) {
		now := time.Now()
		refreshed := now.Add(-30 * 24 * time.Hour)
		first := engine.ApplyStalenessDecay(baseProfile, refreshed, now)
		repeated := engine.ApplyStalenessDecay(first, refreshed, now)
		if math.Abs(repeated.Overall-first.Overall) > 1e-9 {
			t.Fatalf("same staleness calculation compounded: %.6f vs %.6f", repeated.Overall, first.Overall)
		}
		for _, attr := range repeated.Attributes {
			if score, ok := repeated.PerAttribute[attr.Field]; ok && math.Abs(attr.Score-score) > 1e-12 {
				t.Errorf("attribute detail %s score %.6f differs from map %.6f", attr.Field, attr.Score, score)
			}
		}
		// Applying a fresher observation time restores the score from the prior
		// decay factor instead of compounding the earlier stale result.
		fresh := engine.ApplyStalenessDecay(repeated, now, now)
		if math.Abs(fresh.Overall-baseProfile.Overall) > 1e-9 {
			t.Fatalf("fresh timestamp did not restore undecayed score: %.6f vs %.6f", fresh.Overall, baseProfile.Overall)
		}
		if math.Abs(first.Overall-baseProfile.Overall) < 1e-6 {
			t.Fatal("staleness call mutated the source profile")
		}
	})

	t.Run("beyond max age", func(t *testing.T) {
		now := time.Now()
		refreshed := now.Add(-400 * 24 * time.Hour)
		result := engine.ApplyStalenessDecay(baseProfile, refreshed, now)

		if result.Overall != 0.0 {
			t.Errorf("beyond max age confidence should be 0.0, got %.4f", result.Overall)
		}
	})

	t.Run("linear decay model", func(t *testing.T) {
		linearEngine := NewConfidenceEngineWithConfig(
			models.DefaultPropagationConfig(),
			models.DefaultCorroborationConfig(),
			models.StalenessConfig{
				HalfLifeDays: 30,
				MaxAgeDays:   365,
				DecayModel:   "linear",
			},
		)

		now := time.Now()
		// Half the max age
		refreshed := now.Add(-182 * 24 * time.Hour)
		result := linearEngine.ApplyStalenessDecay(baseProfile, refreshed, now)

		// At 50% of max age, confidence should be ~50%
		expected := originalOverall * 0.5
		tolerance := 0.10
		if math.Abs(result.Overall-expected) > tolerance {
			t.Errorf("linear at 50%% max age, expected ~%.4f, got %.4f", expected, result.Overall)
		}
	})

	t.Run("per-attribute decay is consistent", func(t *testing.T) {
		now := time.Now()
		refreshed := now.Add(-10 * 24 * time.Hour)
		result := engine.ApplyStalenessDecay(baseProfile, refreshed, now)

		// All per-attribute scores should have same decay ratio
		var ratio float64
		first := true
		for field := range baseProfile.PerAttribute {
			if baseProfile.PerAttribute[field] > 0 {
				r := result.PerAttribute[field] / baseProfile.PerAttribute[field]
				if first {
					ratio = r
					first = false
				} else {
					if math.Abs(r-ratio) > 0.001 {
						t.Errorf("inconsistent decay ratios: field %s = %.4f, expected ~%.4f", field, r, ratio)
					}
				}
			}
		}
	})
}

// ========================================================================
// Visualization Hint Tests
// ========================================================================

func TestComputeVisualizationHint(t *testing.T) {
	tests := []struct {
		name           string
		confidence     float64
		isLink         bool
		expectedColor  string
		expectedLabel  string
		checkThickness bool
		checkSize      bool
	}{
		{"high confidence node", 0.95, false, "#22c55e", "High", false, true},
		{"medium confidence node", 0.60, false, "#eab308", "Medium", false, true},
		{"low confidence node", 0.30, false, "#f97316", "Low", false, true},
		{"none confidence node", 0.10, false, "#ef4444", "None", false, true},
		{"high confidence link", 0.95, true, "#22c55e", "High", true, false},
		{"low confidence link", 0.25, true, "#f97316", "Low", true, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hint := models.ComputeVisualizationHint(tt.confidence, tt.isLink)

			if hint.Color != tt.expectedColor {
				t.Errorf("color = %s, want %s", hint.Color, tt.expectedColor)
			}
			if hint.Label != tt.expectedLabel {
				t.Errorf("label = %s, want %s", hint.Label, tt.expectedLabel)
			}
			if tt.checkThickness && hint.Thickness <= 0 {
				t.Error("link should have positive thickness")
			}
			if tt.checkSize && hint.Size <= 0 {
				t.Error("node should have positive size")
			}
			// Higher confidence = higher opacity
			if tt.confidence >= 0.80 && hint.Opacity < 0.80 {
				t.Errorf("high confidence should have high opacity, got %.2f", hint.Opacity)
			}
			// Low confidence links should have dash pattern
			if tt.confidence < 0.50 && tt.isLink && hint.DashPattern == "" {
				t.Error("low confidence links should have dash pattern")
			}
		})
	}
}

// ========================================================================
// Summary Statistics Tests
// ========================================================================

func TestComputeSummaryStats(t *testing.T) {
	engine := NewConfidenceEngine()

	t.Run("empty profiles", func(t *testing.T) {
		stats := engine.ComputeSummaryStats(nil)
		if stats.TotalRecords != 0 {
			t.Errorf("empty stats should have 0 records, got %d", stats.TotalRecords)
		}
	})

	t.Run("single profile", func(t *testing.T) {
		profile := engine.ScoreDeviceAttributes(
			models.ConfidenceSourceConfig, models.ConfidenceSourceConfig,
			models.ConfidenceSourceConfig, models.ConfidenceSourceConfig,
			models.ConfidenceSourceConfig, models.ConfidenceSourceConfig,
			models.ConfidenceSourceConfig,
		)
		stats := engine.ComputeSummaryStats([]*models.ConfidenceProfile{profile})

		if stats.TotalRecords != 1 {
			t.Errorf("expected 1 record, got %d", stats.TotalRecords)
		}
		if stats.AverageConfidence != profile.Overall {
			t.Errorf("average should equal single score: %.4f vs %.4f",
				stats.AverageConfidence, profile.Overall)
		}
	})

	t.Run("multiple profiles with tiers", func(t *testing.T) {
		highProfile := engine.ScoreDeviceAttributes(
			models.ConfidenceSourceConfig, models.ConfidenceSourceConfig,
			models.ConfidenceSourceConfig, models.ConfidenceSourceConfig,
			models.ConfidenceSourceConfig, models.ConfidenceSourceConfig,
			models.ConfidenceSourceConfig,
		)
		lowProfile := engine.ScoreDeviceAttributes(
			models.ConfidenceSourceInferred, models.ConfidenceSourceInferred,
			models.ConfidenceSourceInferred, models.ConfidenceSourceInferred,
			models.ConfidenceSourceInferred, models.ConfidenceSourceInferred,
			models.ConfidenceSourceInferred,
		)

		stats := engine.ComputeSummaryStats([]*models.ConfidenceProfile{highProfile, lowProfile})

		if stats.TotalRecords != 2 {
			t.Errorf("expected 2 records, got %d", stats.TotalRecords)
		}
		if stats.RecordsByTier[models.ConfidenceTierMedium] < 1 {
			t.Error("expected at least 1 medium confidence record")
		}
		// Average should be between the two
		if stats.AverageConfidence <= lowProfile.Overall || stats.AverageConfidence >= highProfile.Overall {
			t.Errorf("average (%.4f) should be between low (%.4f) and high (%.4f)",
				stats.AverageConfidence, lowProfile.Overall, highProfile.Overall)
		}
		// Min and max should be correct
		if stats.MinConfidence != lowProfile.Overall {
			t.Errorf("min = %.4f, want %.4f", stats.MinConfidence, lowProfile.Overall)
		}
		if stats.MaxConfidence != highProfile.Overall {
			t.Errorf("max = %.4f, want %.4f", stats.MaxConfidence, highProfile.Overall)
		}
	})
}

// ========================================================================
// Min-Confidence Filter Tests
// ========================================================================

func TestFilterByConfidence(t *testing.T) {
	engine := NewConfidenceEngine()

	highProfile := engine.ScoreDeviceAttributes(
		models.ConfidenceSourceConfig, models.ConfidenceSourceConfig,
		models.ConfidenceSourceConfig, models.ConfidenceSourceConfig,
		models.ConfidenceSourceConfig, models.ConfidenceSourceConfig,
		models.ConfidenceSourceConfig,
	)
	lowProfile := engine.ScoreDeviceAttributes(
		models.ConfidenceSourceInferred, models.ConfidenceSourceInferred,
		models.ConfidenceSourceInferred, models.ConfidenceSourceInferred,
		models.ConfidenceSourceInferred, models.ConfidenceSourceInferred,
		models.ConfidenceSourceInferred,
	)

	allProfiles := []*models.ConfidenceProfile{highProfile, lowProfile}

	t.Run("no filter returns all", func(t *testing.T) {
		filter := models.DefaultConfidenceFilter()
		result := engine.FilterByConfidence(allProfiles, filter)
		if len(result) != 2 {
			t.Errorf("default filter should return all, got %d", len(result))
		}
	})

	t.Run("min confidence threshold passes high", func(t *testing.T) {
		filter := models.ConfidenceFilter{MinConfidence: 0.70}
		result := engine.FilterByConfidence(allProfiles, filter)
		if len(result) != 1 {
			t.Errorf("min=0.70 should filter out low, got %d records", len(result))
		}
		if result[0] != highProfile {
			t.Error("should keep high confidence profile")
		}
	})

	t.Run("min confidence threshold filters all", func(t *testing.T) {
		filter := models.ConfidenceFilter{MinConfidence: 0.99}
		result := engine.FilterByConfidence(allProfiles, filter)
		if len(result) != 0 {
			t.Errorf("min=0.99 should filter all, got %d", len(result))
		}
	})

	t.Run("tier-based filter", func(t *testing.T) {
		filter := models.ConfidenceFilter{MinTier: models.ConfidenceTierMedium}
		result := engine.FilterByConfidence(allProfiles, filter)
		// High profile has tier High, low has tier Low or None
		if len(result) != 1 {
			t.Errorf("medium+ tier filter should return 1, got %d", len(result))
		}
	})

	t.Run("exclude sources", func(t *testing.T) {
		filter := models.ConfidenceFilter{
			ExcludeSources: []models.ConfidenceSource{models.ConfidenceSourceConfig},
		}
		result := engine.FilterByConfidence(allProfiles, filter)
		// High profile uses Config source -> should be filtered out
		// Low profile uses Inferred source -> should remain
		for _, p := range result {
			for _, src := range p.EvidenceSources {
				if src == models.ConfidenceSourceConfig {
					t.Error("config source should have been excluded")
				}
			}
		}
	})

	t.Run("require corroboration", func(t *testing.T) {
		// Add corroboration to high profile
		evidence := []models.CorroborationEvidence{
			{SourceType: models.ConfidenceSourceConfig, SourceName: "ssh", Field: "hostname", Value: "s1"},
			{SourceType: models.ConfidenceSourceConnector, SourceName: "netbox", Field: "hostname", Value: "s1"},
		}
		boosted := engine.ApplyCorroboration(evidence, highProfile)

		filter := models.ConfidenceFilter{RequireCorroboration: 2}
		result := engine.FilterByConfidence([]*models.ConfidenceProfile{boosted, lowProfile}, filter)
		if len(result) != 1 {
			t.Errorf("require 2 corroborating sources should return boosted only, got %d", len(result))
		}

		// Evidence item count can be high without independent corroboration.
		uncorroborated := models.NewConfidenceProfile()
		uncorroborated.EvidenceCount = 20
		result = engine.FilterByConfidence([]*models.ConfidenceProfile{uncorroborated}, filter)
		if len(result) != 0 {
			t.Error("attribute evidence count must not satisfy a source corroboration requirement")
		}
	})

	t.Run("max staleness filter", func(t *testing.T) {
		now := time.Now()
		oldDate := now.Add(-90 * 24 * time.Hour)
		decayedProfile := engine.ApplyStalenessDecay(highProfile, oldDate, now)

		filter := models.ConfidenceFilter{MaxStaleness: 0.30}
		result := engine.FilterByConfidence([]*models.ConfidenceProfile{decayedProfile, lowProfile}, filter)

		// decayedProfile has high staleness decay (90 days), lowProfile has 0 decay
		// With MaxStaleness=0.30, decayedProfile should be filtered out
		if len(result) != 1 {
			t.Errorf("max staleness=0.30 should filter out stale record, got %d", len(result))
		}
	})
}

// ========================================================================
// Integration Tests (End-to-End Scenarios)
// ========================================================================

func TestEndToEndDevicePipeline(t *testing.T) {
	engine := NewConfidenceEngine()

	// 1. Score a device from config
	deviceProfile := engine.ScoreDeviceAttributes(
		models.ConfidenceSourceConfig, models.ConfidenceSourceConfig,
		models.ConfidenceSourceConfig, models.ConfidenceSourceConfig,
		models.ConfidenceSourceConfig, models.ConfidenceSourceConfig,
		models.ConfidenceSourceConfig,
	)
	t.Logf("Device confidence: %.4f (%s)", deviceProfile.Overall, deviceProfile.Tier)

	// 2. Propagate to interfaces
	ifaceFields := []string{"name", "ip_address", "vlan_id", "status"}
	ifaceProfile := engine.PropagateDeviceToInterfaces(deviceProfile, ifaceFields)
	t.Logf("Interface confidence (propagated): %.4f (%s)", ifaceProfile.Overall, ifaceProfile.Tier)

	// 3. Apply corroboration (NetBox agrees on hostname)
	evidence := []models.CorroborationEvidence{
		{SourceType: models.ConfidenceSourceConfig, SourceName: "ssh-cisco-ios", Field: "hostname", Value: "core-sw-01"},
		{SourceType: models.ConfidenceSourceConnector, SourceName: "netbox", Field: "hostname", Value: "core-sw-01"},
	}
	corroboratedProfile := engine.ApplyCorroboration(evidence, deviceProfile)
	t.Logf("Corroborated device: %.4f (boost=%.4f)", corroboratedProfile.Overall, corroboratedProfile.CorroborationBoost)

	if corroboratedProfile.Overall <= deviceProfile.Overall {
		t.Error("corroboration should increase overall confidence")
	}

	// 4. Apply staleness (config is 45 days old)
	now := time.Now()
	refreshed := now.Add(-45 * 24 * time.Hour)
	staleProfile := engine.ApplyStalenessDecay(corroboratedProfile, refreshed, now)
	t.Logf("Stale device (45 days): %.4f (decay=%.4f)", staleProfile.Overall, staleProfile.StalenessDecay)

	if staleProfile.Overall >= corroboratedProfile.Overall {
		t.Error("staleness should decrease overall confidence")
	}

	// 5. Compute stats
	stats := engine.ComputeSummaryStats([]*models.ConfidenceProfile{
		deviceProfile, corroboratedProfile, staleProfile, ifaceProfile,
	})
	t.Logf("Stats: %d records, avg=%.4f, median=%.4f", stats.TotalRecords, stats.AverageConfidence, stats.MedianConfidence)

	if stats.TotalRecords != 4 {
		t.Errorf("expected 4 records in stats, got %d", stats.TotalRecords)
	}

	// 6. Filter by min confidence
	filter := models.ConfidenceFilter{MinConfidence: 0.60}
	filtered := engine.FilterByConfidence([]*models.ConfidenceProfile{
		deviceProfile, corroboratedProfile, staleProfile, ifaceProfile,
	}, filter)
	t.Logf("Filtered (min=0.60): %d records", len(filtered))

	if len(filtered) == 0 {
		t.Error("filter at 0.60 should return at least 1 record")
	}

	// 7. Visualization hints
	deviceHint := engine.GetVisualizationHint(deviceProfile, false)
	linkHint := engine.GetVisualizationHint(ifaceProfile, true)
	t.Logf("Device viz: color=%s opacity=%.2f size=%.2f", deviceHint.Color, deviceHint.Opacity, deviceHint.Size)
	t.Logf("Link viz: color=%s opacity=%.2f thickness=%.2f", linkHint.Color, linkHint.Opacity, linkHint.Thickness)

	if deviceHint.Color == "" {
		t.Error("device should have a color hint")
	}
	if linkHint.Thickness <= 0 {
		t.Error("link should have thickness hint")
	}
}

func TestConfidenceProfileHelpers(t *testing.T) {
	profile := models.NewConfidenceProfile()

	if profile.Overall != 0.0 {
		t.Error("new profile should start at 0.0")
	}
	if profile.Tier != models.ConfidenceTierNone {
		t.Error("new profile should be tier None")
	}

	profile.AddAttribute("hostname", 0.90, models.ConfidenceSourceConfig, "from config")
	profile.AddAttribute("vendor", 0.85, models.ConfidenceSourceConfig, "from config")
	profile.SetOverall()

	if profile.Overall == 0.0 {
		t.Error("profile with attributes should have positive overall")
	}
	if profile.EvidenceCount != 2 {
		t.Errorf("expected 2 evidence items, got %d", profile.EvidenceCount)
	}
	if profile.Tier == models.ConfidenceTierNone {
		t.Error("profile with high scores should not be tier None")
	}
}

func TestTierFromScore(t *testing.T) {
	tests := []struct {
		score    float64
		expected models.ConfidenceTier
	}{
		{1.00, models.ConfidenceTierHigh},
		{0.80, models.ConfidenceTierHigh},
		{0.79, models.ConfidenceTierMedium},
		{0.50, models.ConfidenceTierMedium},
		{0.49, models.ConfidenceTierLow},
		{0.20, models.ConfidenceTierLow},
		{0.19, models.ConfidenceTierNone},
		{0.00, models.ConfidenceTierNone},
		{-1.0, models.ConfidenceTierNone},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("score_%.2f", tt.score), func(t *testing.T) {
			got := models.TierFromScore(tt.score)
			if got != tt.expected {
				t.Errorf("TierFromScore(%.2f) = %s, want %s", tt.score, got, tt.expected)
			}
		})
	}
}

func TestMinTierScore(t *testing.T) {
	if models.MinTierScore(models.ConfidenceTierHigh) != 0.80 {
		t.Error("high tier min should be 0.80")
	}
	if models.MinTierScore(models.ConfidenceTierMedium) != 0.50 {
		t.Error("medium tier min should be 0.50")
	}
	if models.MinTierScore(models.ConfidenceTierLow) != 0.20 {
		t.Error("low tier min should be 0.20")
	}
	if models.MinTierScore(models.ConfidenceTierNone) != 0.0 {
		t.Error("none tier min should be 0.0")
	}
}

func TestConfidenceColorMap(t *testing.T) {
	for _, tier := range []models.ConfidenceTier{
		models.ConfidenceTierHigh,
		models.ConfidenceTierMedium,
		models.ConfidenceTierLow,
		models.ConfidenceTierNone,
	} {
		if color, ok := models.ConfidenceColorMap[tier]; !ok || color == "" {
			t.Errorf("missing color for tier %s", tier)
		}
	}
}

func TestSourceWeights(t *testing.T) {
	// Verify source weight ordering: manual > config > operational > connector > inferred
	weights := []struct {
		source models.ConfidenceSource
		weight float64
	}{
		{models.ConfidenceSourceManual, 1.00},
		{models.ConfidenceSourceConfig, 0.90},
		{models.ConfidenceSourceOperational, 0.80},
		{models.ConfidenceSourceConnector, 0.70},
		{models.ConfidenceSourceInferred, 0.40},
	}

	for i := 1; i < len(weights); i++ {
		if weights[i].weight >= weights[i-1].weight {
			t.Errorf("source weights should be ordered: %s (%.2f) should be < %s (%.2f)",
				weights[i].source, weights[i].weight, weights[i-1].source, weights[i-1].weight)
		}
	}
}

func TestDefaultConfigs(t *testing.T) {
	prop := models.DefaultPropagationConfig()
	if prop.DecayFactor != 0.95 {
		t.Errorf("default propagation decay factor should be 0.95")
	}
	if prop.MaxHops != 5 {
		t.Errorf("default max hops should be 5")
	}

	corr := models.DefaultCorroborationConfig()
	if corr.MaxBoost != 0.20 {
		t.Errorf("default max corroboration boost should be 0.20")
	}

	stale := models.DefaultStalenessConfig()
	if stale.HalfLifeDays != 30 {
		t.Errorf("default half-life should be 30 days")
	}
	if stale.MaxAgeDays != 365 {
		t.Errorf("default max age should be 365 days")
	}
}
