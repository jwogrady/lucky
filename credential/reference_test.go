package credential

import "testing"

func TestValidateReference(t *testing.T) {
	valid := []string{
		"op://wtp/item/password",
		"op://wtp/item/section/password",
		// Field labels in the wtp vault contain spaces; that is legal.
		"op://wtp/Google service account wtp-analytics-reader/credential",
	}
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
