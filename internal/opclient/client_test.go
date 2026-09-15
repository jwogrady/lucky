package opclient

import (
	"errors"
	"strings"
	"testing"
)

func TestValidateReference(t *testing.T) {
	valid := []string{"op://wtp/item/password", "op://wtp/item/section/password"}
	for _, ref := range valid {
		if err := ValidateReference(ref); err != nil {
			t.Errorf("%s: %v", ref, err)
		}
	}
	invalid := []string{"wtp/item/password", "op://wtp/item", "op://wtp//password", "op://wtp/item/password?x=y", "op://wtp/item/../password"}
	for _, ref := range invalid {
		if err := ValidateReference(ref); err == nil {
			t.Errorf("expected %s to fail", ref)
		}
	}
}
func TestSafeErrorRedactsSensitiveInputs(t *testing.T) {
	err := safeError("failed", errors.New("token abc and op://wtp/item/password"), "abc", "op://wtp/item/password")
	if strings.Contains(err.Error(), "abc") || strings.Contains(err.Error(), "op://") {
		t.Fatalf("unsafe error: %s", err)
	}
}
