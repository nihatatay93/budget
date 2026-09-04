package category

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// IconType identifies whether IconValue is a semantic system-icon key or a Unicode emoji.
// Platform-specific icon names never cross this boundary.
type IconType string

const (
	IconTypeSystem IconType = "system"
	IconTypeEmoji  IconType = "emoji"
)

type Appearance struct {
	IconType  IconType
	IconValue string
	ColorKey  string
}

const (
	defaultSystemIcon = "ellipsis"
	defaultColorKey   = "slate"
)

var supportedSystemIcons = map[string]struct{}{
	"home": {}, "shopping-cart": {}, "utensils": {}, "car": {}, "receipt": {},
	"shopping-bag": {}, "heart": {}, "gamepad": {}, "repeat": {}, "plane": {},
	"graduation-cap": {}, "sparkles": {}, "gift": {}, "ellipsis": {}, "wallet": {},
	"laptop": {}, "trending-up": {}, "building": {}, "refund": {}, "wallet-more": {},
}

var supportedColorKeys = map[string]struct{}{
	"green": {}, "mint": {}, "blue": {}, "cyan": {}, "purple": {}, "pink": {},
	"red": {}, "orange": {}, "amber": {}, "slate": {},
}

func DefaultAppearance() Appearance {
	return Appearance{IconType: IconTypeSystem, IconValue: defaultSystemIcon, ColorKey: defaultColorKey}
}

func IsSupportedSystemIcon(value string) bool {
	_, ok := supportedSystemIcons[value]
	return ok
}

func IsSupportedColorKey(value string) bool {
	_, ok := supportedColorKeys[value]
	return ok
}

func (a Appearance) Valid() bool {
	if !IsSupportedColorKey(a.ColorKey) {
		return false
	}
	switch a.IconType {
	case IconTypeSystem:
		return IsSupportedSystemIcon(a.IconValue)
	case IconTypeEmoji:
		return validEmoji(a.IconValue)
	default:
		return false
	}
}

// normalizeAppearance accepts the deprecated free-form icon only as a compatibility input.
// New callers must provide all three typed fields together; existing callers safely fall back.
func normalizeAppearance(iconType, iconValue, colorKey, legacyIcon *string) (Appearance, error) {
	if iconType == nil && iconValue == nil && colorKey == nil {
		if legacyIcon == nil || strings.TrimSpace(*legacyIcon) == "" {
			return DefaultAppearance(), nil
		}
		value := strings.TrimSpace(*legacyIcon)
		if IsSupportedSystemIcon(value) {
			return Appearance{IconType: IconTypeSystem, IconValue: value, ColorKey: defaultColorKey}, nil
		}
		appearance := Appearance{IconType: IconTypeEmoji, IconValue: value, ColorKey: defaultColorKey}
		if !appearance.Valid() {
			return Appearance{}, ErrInvalidInput
		}
		return appearance, nil
	}
	if iconType == nil || iconValue == nil || colorKey == nil {
		return Appearance{}, ErrInvalidInput
	}
	appearance := Appearance{
		IconType:  IconType(strings.TrimSpace(*iconType)),
		IconValue: strings.TrimSpace(*iconValue),
		ColorKey:  strings.TrimSpace(*colorKey),
	}
	if !appearance.Valid() {
		return Appearance{}, ErrInvalidInput
	}
	return appearance, nil
}

// validEmoji validates Unicode emoji sequences without assuming a single rune or grapheme.
// It accepts ZWJ, variation-selector, keycap, and skin-tone components around emoji scalars.
func validEmoji(value string) bool {
	if value == "" || len(value) > 64 || !utf8.ValidString(value) {
		return false
	}
	hasEmoji := false
	for _, r := range value {
		switch {
		case isEmojiScalar(r):
			hasEmoji = true
		case r == 0x200D || r == 0xFE0F || r == 0x20E3 || (r >= '0' && r <= '9') || r == '#' || r == '*':
			// Valid only as components of an emoji sequence; hasEmoji below remains required.
		case unicode.IsControl(r) || unicode.IsSpace(r):
			return false
		default:
			return false
		}
	}
	return hasEmoji
}

func isEmojiScalar(r rune) bool {
	return (r >= 0x1F000 && r <= 0x1FAFF) ||
		(r >= 0x2600 && r <= 0x27BF) ||
		(r >= 0x2300 && r <= 0x23FF)
}
