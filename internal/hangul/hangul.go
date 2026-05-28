package hangul

import (
	"fmt"
	"strconv"
	"strings"
)

const (
	hangulBase = 0xAC00
	hangulEnd  = 0xD7A3
	jungCount  = 21
	jongCount  = 28
)

var choseong = []string{
	"ㄱ", "ㄲ", "ㄴ", "ㄷ", "ㄸ", "ㄹ", "ㅁ", "ㅂ", "ㅃ", "ㅅ",
	"ㅆ", "ㅇ", "ㅈ", "ㅉ", "ㅊ", "ㅋ", "ㅌ", "ㅍ", "ㅎ",
}

var jungseong = []string{
	"ㅏ", "ㅐ", "ㅑ", "ㅒ", "ㅓ", "ㅔ", "ㅕ", "ㅖ", "ㅗ", "ㅘ",
	"ㅙ", "ㅚ", "ㅛ", "ㅜ", "ㅝ", "ㅞ", "ㅟ", "ㅠ", "ㅡ", "ㅢ", "ㅣ",
}

var jongseong = []string{
	"", "ㄱ", "ㄲ", "ㄳ", "ㄴ", "ㄵ", "ㄶ", "ㄷ", "ㄹ", "ㄺ",
	"ㄻ", "ㄼ", "ㄽ", "ㄾ", "ㄿ", "ㅀ", "ㅁ", "ㅂ", "ㅄ", "ㅅ",
	"ㅆ", "ㅇ", "ㅈ", "ㅊ", "ㅋ", "ㅌ", "ㅍ", "ㅎ",
}

type Parts struct {
	Choseong       string
	Jungseong      string
	Jongseong      string
	ChoseongIndex  int
	JungseongIndex int
	JongseongIndex int
}

func (p Parts) HasJongseong() bool {
	return p.JongseongIndex > 0
}

func DecomposeRune(r rune) (Parts, bool) {
	if r < hangulBase || r > hangulEnd {
		return Parts{}, false
	}
	offset := int(r - hangulBase)
	choseongIndex := offset / (jungCount * jongCount)
	jungseongIndex := (offset % (jungCount * jongCount)) / jongCount
	jongseongIndex := offset % jongCount
	return Parts{
		Choseong:       choseong[choseongIndex],
		Jungseong:      jungseong[jungseongIndex],
		Jongseong:      jongseong[jongseongIndex],
		ChoseongIndex:  choseongIndex,
		JungseongIndex: jungseongIndex,
		JongseongIndex: jongseongIndex,
	}, true
}

func Compose(choseongIndex int, jungseongIndex int, jongseongIndex int) rune {
	return rune(hangulBase + ((choseongIndex*jungCount)+jungseongIndex)*jongCount + jongseongIndex)
}

func ChoseongTokens() []string {
	return append([]string(nil), choseong...)
}

func JungseongTokens() []string {
	return append([]string(nil), jungseong...)
}

func JongseongTokens() []string {
	return append([]string(nil), jongseong[1:]...)
}

func SyllablePromptID(r rune) string {
	return fmt.Sprintf("syllable-%04X", r)
}

func ChoseongPromptID(token string) string {
	return promptID("jamo-choseong", token)
}

func JungseongPromptID(token string) string {
	return promptID("jamo-jungseong", token)
}

func JongseongPromptID(token string) string {
	if token == "" {
		return ""
	}
	return promptID("jamo-jongseong", token)
}

func JamoPromptIDs(parts Parts) []string {
	ids := []string{
		ChoseongPromptID(parts.Choseong),
		JungseongPromptID(parts.Jungseong),
	}
	if parts.HasJongseong() {
		ids = append(ids, JongseongPromptID(parts.Jongseong))
	}
	return ids
}

func TokenFromPromptID(id string) string {
	parts := strings.Split(id, "-")
	if len(parts) < 3 {
		return ""
	}
	code, err := strconv.ParseInt(parts[len(parts)-1], 16, 32)
	if err != nil {
		return ""
	}
	return string(rune(code))
}

func promptID(prefix string, token string) string {
	runes := []rune(token)
	if len(runes) == 0 {
		return prefix + "-none"
	}
	return fmt.Sprintf("%s-%04X", prefix, runes[0])
}
