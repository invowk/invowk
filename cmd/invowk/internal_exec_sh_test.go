// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"slices"
	"testing"
)

func TestBuildShEnvUsesOnlySerializedAndExplicitEnv(t *testing.T) {
	t.Setenv("INVOWK_TEST_SHOULD_NOT_LEAK", "host-value")

	env, err := buildShEnv([]string{"EXPLICIT=1"}, `{"SERIALIZED":"2"}`)
	if err != nil {
		t.Fatalf("buildShEnv() error = %v", err)
	}

	if !slices.Contains(env, "EXPLICIT=1") {
		t.Fatalf("buildShEnv() = %v, want explicit env", env)
	}
	if !slices.Contains(env, "SERIALIZED=2") {
		t.Fatalf("buildShEnv() = %v, want serialized env", env)
	}
	if slices.Contains(env, "INVOWK_TEST_SHOULD_NOT_LEAK=host-value") {
		t.Fatalf("buildShEnv() leaked host process env: %v", env)
	}
}
