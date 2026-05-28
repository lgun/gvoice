package hangul

import "testing"

func TestDecomposeRune(t *testing.T) {
	parts, ok := DecomposeRune('한')
	if !ok {
		t.Fatal("expected Hangul syllable")
	}
	if parts.Choseong != "ㅎ" || parts.Jungseong != "ㅏ" || parts.Jongseong != "ㄴ" {
		t.Fatalf("unexpected parts: %#v", parts)
	}
	if got := Compose(parts.ChoseongIndex, parts.JungseongIndex, parts.JongseongIndex); got != '한' {
		t.Fatalf("compose mismatch: got %q", got)
	}
}
