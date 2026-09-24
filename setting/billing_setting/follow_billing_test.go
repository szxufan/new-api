package billing_setting

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func withFollowMap(t *testing.T, follows map[string]FollowConfig) {
	t.Helper()
	saved := billingSetting.BillingFollow
	billingSetting.BillingFollow = follows
	t.Cleanup(func() {
		billingSetting.BillingFollow = saved
	})
}

func TestResolveFollowDirect(t *testing.T) {
	withFollowMap(t, map[string]FollowConfig{
		"a": {TargetModel: "b", Coefficient: 1.5},
	})

	target, coefficient, ok := ResolveFollow("a")
	require.True(t, ok)
	require.Equal(t, "b", target)
	require.InDelta(t, 1.5, coefficient, 1e-9)
}

func TestResolveFollowChainMultipliesCoefficients(t *testing.T) {
	withFollowMap(t, map[string]FollowConfig{
		"a": {TargetModel: "b", Coefficient: 0.5},
		"b": {TargetModel: "c", Coefficient: 2.0},
	})

	target, coefficient, ok := ResolveFollow("a")
	require.True(t, ok)
	require.Equal(t, "c", target)
	require.InDelta(t, 1.0, coefficient, 1e-9)
}

func TestResolveFollowDefaultCoefficient(t *testing.T) {
	withFollowMap(t, map[string]FollowConfig{
		"a": {TargetModel: "b", Coefficient: 0},
	})

	target, coefficient, ok := ResolveFollow("a")
	require.True(t, ok)
	require.Equal(t, "b", target)
	require.InDelta(t, 1.0, coefficient, 1e-9)
}

func TestResolveFollowSelfCycle(t *testing.T) {
	withFollowMap(t, map[string]FollowConfig{
		"a": {TargetModel: "a", Coefficient: 1},
	})

	_, _, ok := ResolveFollow("a")
	require.False(t, ok)
}

func TestResolveFollowTwoNodeCycle(t *testing.T) {
	withFollowMap(t, map[string]FollowConfig{
		"a": {TargetModel: "b", Coefficient: 1},
		"b": {TargetModel: "a", Coefficient: 1},
	})

	_, _, ok := ResolveFollow("a")
	require.False(t, ok)
}

func TestResolveFollowDepthLimit(t *testing.T) {
	follows := make(map[string]FollowConfig)
	for i := 0; i < 20; i++ {
		follows[string(rune('a'+i))] = FollowConfig{TargetModel: string(rune('a' + i + 1))}
	}
	withFollowMap(t, follows)

	target, coefficient, ok := ResolveFollow("a")
	require.True(t, ok)
	require.Equal(t, string(rune('a'+maxFollowDepth)), target)
	require.InDelta(t, 1.0, coefficient, 1e-9)
}

func TestResolveFollowUnset(t *testing.T) {
	withFollowMap(t, map[string]FollowConfig{})

	_, _, ok := ResolveFollow("a")
	require.False(t, ok)
}

func TestResolveFollowEmptyTarget(t *testing.T) {
	withFollowMap(t, map[string]FollowConfig{
		"a": {TargetModel: "  ", Coefficient: 1},
	})

	_, _, ok := ResolveFollow("a")
	require.False(t, ok)
}
