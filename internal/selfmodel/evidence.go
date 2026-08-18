package selfmodel

import "deep-seeing/internal/memory"

// EvidenceWeight returns relative strength of an experience mode (0..1 scale hint).
func EvidenceWeight(mode memory.ExperienceMode) float64 {
	switch memory.NormalizeExperienceMode(string(mode)) {
	case memory.ExperienceRealInteraction:
		return 1.0
	case memory.ExperienceSelfReflection:
		return 0.55
	case memory.ExperienceExternalObservation:
		return 0.5
	case memory.ExperienceSimulatedRoleplay, memory.ExperienceStoryReading:
		return 0.35
	default:
		return 0.4
	}
}

// CanPromoteToPrinciple reports whether modes alone can support a claimed principle.
// Roleplay/story-only evidence is never enough.
func CanPromoteToPrinciple(modes []memory.ExperienceMode) bool {
	strong := 0
	for _, m := range modes {
		switch memory.NormalizeExperienceMode(string(m)) {
		case memory.ExperienceRealInteraction:
			strong++
		case memory.ExperienceSimulatedRoleplay, memory.ExperienceStoryReading:
			// weak — ignored for promotion
		default:
			// medium does not alone promote
		}
	}
	return strong >= 1
}

// MaxStatusForModes caps status given evidence modes.
func MaxStatusForModes(modes []memory.ExperienceMode) Status {
	if CanPromoteToPrinciple(modes) {
		return StatusClaimed
	}
	onlyWeak := len(modes) > 0
	for _, m := range modes {
		nm := memory.NormalizeExperienceMode(string(m))
		if nm != memory.ExperienceSimulatedRoleplay && nm != memory.ExperienceStoryReading {
			onlyWeak = false
			break
		}
	}
	if onlyWeak {
		return StatusTentative
	}
	return StatusTentative
}
